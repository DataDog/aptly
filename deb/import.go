package deb

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/DataDog/aptly/aptly"
	"github.com/DataDog/aptly/pgp"
	"github.com/DataDog/aptly/utils"
)

// CollectPackageFiles walks filesystem collecting all candidates for package files
func CollectPackageFiles(locations []string, reporter aptly.ResultReporter) (packageFiles, otherFiles, failedFiles []string) {
	for _, location := range locations {
		info, err2 := os.Stat(location)
		if err2 != nil {
			reporter.Warning("Unable to process %s: %s", location, err2)
			failedFiles = append(failedFiles, location)
			continue
		}
		if info.IsDir() {
			err2 = filepath.Walk(location, func(path string, info os.FileInfo, err3 error) error {
				if err3 != nil {
					return err3
				}
				if info.IsDir() {
					return nil
				}

				if strings.HasSuffix(info.Name(), ".deb") || strings.HasSuffix(info.Name(), ".udeb") ||
					strings.HasSuffix(info.Name(), ".dsc") || strings.HasSuffix(info.Name(), ".ddeb") {
					packageFiles = append(packageFiles, path)
				} else if strings.HasSuffix(info.Name(), ".buildinfo") {
					otherFiles = append(otherFiles, path)
				}

				return nil
			})

			if err2 != nil {
				reporter.Warning("Unable to process %s: %s", location, err2)
				failedFiles = append(failedFiles, location)
				continue
			}
		} else {
			if strings.HasSuffix(info.Name(), ".deb") || strings.HasSuffix(info.Name(), ".udeb") ||
				strings.HasSuffix(info.Name(), ".dsc") || strings.HasSuffix(info.Name(), ".ddeb") {
				packageFiles = append(packageFiles, location)
			} else if strings.HasSuffix(info.Name(), ".buildinfo") {
				otherFiles = append(otherFiles, location)
			} else {
				reporter.Warning("Unknown file extension: %s", location)
				failedFiles = append(failedFiles, location)
				continue
			}
		}
	}

	sort.Strings(packageFiles)

	return
}

// ImportPackageFiles imports files into local repository
func ImportPackageFiles(list *PackageList, packageFiles []string, forceReplace bool, verifier pgp.Verifier,
	pool aptly.PackagePool, collection *PackageCollection, reporter aptly.ResultReporter, restriction PackageQuery,
	checksumStorageProvider aptly.ChecksumStorageProvider) (processedFiles []string, failedFiles []string, err error) {
	if forceReplace {
		list.PrepareIndex()
	}

	transaction, err := collection.db.OpenTransaction()
	if err != nil {
		return nil, nil, err
	}
	defer transaction.Discard()

	checksumStorage := checksumStorageProvider(transaction)

	for _, file := range packageFiles {
		var (
			stanza Stanza
			p      *Package
		)

		candidateProcessedFiles := []string{}
		isSourcePackage := strings.HasSuffix(file, ".dsc")
		isUdebPackage := strings.HasSuffix(file, ".udeb")

		if isSourcePackage {
			stanza, err = GetControlFileFromDsc(file, verifier)

			if err == nil {
				stanza.Set("Package", stanza.Get("Source"))
				delete(stanza, "Source")

				p, err = NewSourcePackageFromControlFile(stanza)
			}
		} else {
			stanza, err = GetControlFileFromDeb(file)
			if isUdebPackage {
				p = NewUdebPackageFromControlFile(stanza)
			} else {
				p = NewPackageFromControlFile(stanza)
			}
		}
		if err != nil {
			reporter.Warning("Unable to read file %s: %s", file, err)
			failedFiles = append(failedFiles, file)
			continue
		}

		if p.Name == "" {
			reporter.Warning("Empty package name on %s", file)
			failedFiles = append(failedFiles, file)
			continue
		}

		if p.Version == "" {
			reporter.Warning("Empty version on %s", file)
			failedFiles = append(failedFiles, file)
			continue
		}

		if p.Architecture == "" {
			reporter.Warning("Empty architecture on %s", file)
			failedFiles = append(failedFiles, file)
			continue
		}

		var files PackageFiles

		if isSourcePackage {
			files = p.Files()
		}

		var checksums utils.ChecksumInfo
		checksums, err = utils.ChecksumsForFile(file)
		if err != nil {
			return nil, nil, err
		}

		mainPackageFile := PackageFile{
			Filename:  filepath.Base(file),
			Checksums: checksums,
		}

		mainPackageFile.PoolPath, err = pool.Import(file, mainPackageFile.Filename, &mainPackageFile.Checksums, false, checksumStorage)
		if err != nil {
			reporter.Warning("Unable to import file %s into pool: %s", file, err)
			failedFiles = append(failedFiles, file)
			continue
		}

		candidateProcessedFiles = append(candidateProcessedFiles, file)

		// go over all the other files
		for i := range files {
			sourceFile := filepath.Join(filepath.Dir(file), filepath.Base(files[i].Filename))

			_, err = os.Stat(sourceFile)
			if err == nil {
				files[i].PoolPath, err = pool.Import(sourceFile, files[i].Filename, &files[i].Checksums, false, checksumStorage)
				if err == nil {
					candidateProcessedFiles = append(candidateProcessedFiles, sourceFile)
				}
			} else if os.IsNotExist(err) {
				// if file is not present, try to find it in the pool
				var (
					err2  error
					found bool
				)

				files[i].PoolPath, found, err2 = pool.Verify("", files[i].Filename, &files[i].Checksums, checksumStorage)
				if err2 != nil {
					err = err2
				} else if found {
					// clear error, file is already in the package pool
					err = nil
				}
			}

			if err != nil {
				reporter.Warning("Unable to import file %s into pool: %s", sourceFile, err)
				failedFiles = append(failedFiles, file)
				break
			}
		}
		if err != nil {
			// some files haven't been imported
			continue
		}

		p.UpdateFiles(append(files, mainPackageFile))

		if restriction != nil && !restriction.Matches(p) {
			reporter.Warning("%s has been ignored as it doesn't match restriction", p)
			failedFiles = append(failedFiles, file)
			continue
		}

		err = collection.UpdateInTransaction(p, transaction)
		if err != nil {
			reporter.Warning("Unable to save package %s: %s", p, err)
			failedFiles = append(failedFiles, file)
			continue
		}

		if forceReplace {
			conflictingPackages := list.Search(Dependency{Pkg: p.Name, Version: p.Version, Relation: VersionEqual, Architecture: p.Architecture}, true)
			for _, cp := range conflictingPackages {
				reporter.Removed("%s removed due to conflict with package being added", cp)
				list.Remove(cp)
			}
		}

		err = list.Add(p)
		if err != nil {
			reporter.Warning("Unable to add package to repo %s: %s", p, err)
			failedFiles = append(failedFiles, file)
			continue
		}

		reporter.Added("%s added", p)
		processedFiles = append(processedFiles, candidateProcessedFiles...)
	}

	err = transaction.Commit()
	return
}
