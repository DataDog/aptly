// Package graph renders the aptly object database as a graphviz graph.
//
// It is kept out of package deb so that importing deb does not pull in gographviz.
package graph

import (
	"fmt"
	"strings"

	"github.com/aptly-dev/aptly/deb"
	"github.com/awalterschulze/gographviz"
)

// BuildGraph generates graph contents from aptly object database
func BuildGraph(collectionFactory *deb.CollectionFactory, layout string) (gographviz.Interface, error) {
	var err error

	graph := gographviz.NewEscape()
	_ = graph.SetDir(true)
	_ = graph.SetName("aptly")

	var labelStart string
	var labelEnd string

	switch layout {
	case "vertical":
		_ = graph.AddAttr("aptly", "rankdir", "LR")
		labelStart = ""
		labelEnd = ""
	case "horizontal":
		fallthrough
	default:
		labelStart = "{"
		labelEnd = "}"
	}

	existingNodes := map[string]bool{}

	err = collectionFactory.RemoteRepoCollection().ForEach(func(repo *deb.RemoteRepo) error {
		e := collectionFactory.RemoteRepoCollection().LoadComplete(repo)
		if e != nil {
			return e
		}

		_ = graph.AddNode("aptly", repo.UUID, map[string]string{
			"shape":     "Mrecord",
			"style":     "filled",
			"fillcolor": "darkgoldenrod1",
			"label": fmt.Sprintf("%sMirror %s|url: %s|dist: %s|comp: %s|arch: %s|pkgs: %d%s", labelStart, repo.Name, repo.ArchiveRoot,
				repo.Distribution, strings.Join(repo.Components, ", "),
				strings.Join(repo.Architectures, ", "), repo.NumPackages(), labelEnd),
		})
		existingNodes[repo.UUID] = true
		return nil
	})

	if err != nil {
		return nil, err
	}

	err = collectionFactory.LocalRepoCollection().ForEach(func(repo *deb.LocalRepo) error {
		e := collectionFactory.LocalRepoCollection().LoadComplete(repo)
		if e != nil {
			return e
		}

		_ = graph.AddNode("aptly", repo.UUID, map[string]string{
			"shape":     "Mrecord",
			"style":     "filled",
			"fillcolor": "mediumseagreen",
			"label": fmt.Sprintf("%sRepo %s|comment: %s|pkgs: %d%s", labelStart,
				repo.Name, repo.Comment, repo.NumPackages(), labelEnd),
		})
		existingNodes[repo.UUID] = true
		return nil
	})

	if err != nil {
		return nil, err
	}

	_ = collectionFactory.SnapshotCollection().ForEach(func(snapshot *deb.Snapshot) error {
		existingNodes[snapshot.UUID] = true
		return nil
	})

	err = collectionFactory.SnapshotCollection().ForEach(func(snapshot *deb.Snapshot) error {
		e := collectionFactory.SnapshotCollection().LoadComplete(snapshot)
		if e != nil {
			return e
		}

		description := snapshot.Description
		if snapshot.SourceKind == deb.SourceRemoteRepo {
			description = "Snapshot from repo"
		}

		_ = graph.AddNode("aptly", snapshot.UUID, map[string]string{
			"shape":     "Mrecord",
			"style":     "filled",
			"fillcolor": "cadetblue1",
			"label": fmt.Sprintf("%sSnapshot %s|%s|pkgs: %d%s", labelStart,
				snapshot.Name, description, snapshot.NumPackages(), labelEnd),
		})

		if snapshot.SourceKind == deb.SourceRemoteRepo || snapshot.SourceKind == deb.SourceLocalRepo || snapshot.SourceKind == deb.SourceSnapshot {
			for _, uuid := range snapshot.SourceIDs {
				_, exists := existingNodes[uuid]
				if exists {
					_ = graph.AddEdge(uuid, snapshot.UUID, true, nil)
				}
			}
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	_ = collectionFactory.PublishedRepoCollection().ForEach(func(repo *deb.PublishedRepo) error {
		_ = graph.AddNode("aptly", repo.UUID, map[string]string{
			"shape":     "Mrecord",
			"style":     "filled",
			"fillcolor": "darkolivegreen1",
			"label": fmt.Sprintf("%sPublished %s|comp: %s|arch: %s%s", labelStart,
				repo.GetPath(), strings.Join(repo.Components(), " "),
				strings.Join(repo.Architectures, ", "), labelEnd),
		})

		for _, uuid := range repo.Sources {
			_, exists := existingNodes[uuid]
			if exists {
				_ = graph.AddEdge(uuid, repo.UUID, true, nil)
			}
		}

		return nil
	})

	return graph, nil
}
