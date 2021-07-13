package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"strings"

	"github.com/coreos/go-semver/semver"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func main() {
	flag.Parse()
	a := flag.Arg(0)
	if a == "" {
		a = "."
	}
	err := stats(os.Stdout, a)
	if err != nil {
		log.Fatal(err)
	}
}

type Tag struct {
	name   string
	commit *object.Commit
}

func stats(w io.Writer, repo string) error {
	r, err := git.PlainOpen(repo)
	if err != nil {
		return err
	}
	store := r.Storer
	tgs, err := r.Tags()
	if err != nil {
		return err
	}
	var tags []*Tag
	err = tgs.ForEach(func(r *plumbing.Reference) error {
		tg, err := object.GetCommit(store, r.Hash())
		if err != nil {
			return err
		}
		tags = append(tags, &Tag{
			name:   r.Name().Short(),
			commit: tg,
		})
		return nil
	})
	if err != nil {
		return err
	}
	sort.SliceStable(tags, func(i, j int) bool {
		a := tags[i].name
		b := tags[j].name
		av := semver.New(a[1:])
		bv := semver.New(b[1:])
		return av.LessThan(*bv)
	})
	e := len(tags) - 1
	start, end := tags[e], tags[e-1]
	var commits []*object.Commit
	err = object.NewCommitPreorderIter(start.commit, nil, nil).ForEach(func(c *object.Commit) error {
		if c.Hash.String() == end.commit.Hash.String() {
			return io.EOF
		}
		commits = append(commits, c)
		return nil
	})
	if err != nil {
		if !errors.Is(err, io.EOF) {
			return err
		}
	}
	format(w, start.name, commits, end, start)
	return nil
}

type info struct {
	Contributors  int
	Contributions int
	Committers    []string
}

const ts = "Mon Jan _2 2006"

func format(w io.Writer, name string, commits []*object.Commit, start, end *Tag) {
	x := calc(commits)
	fmt.Fprintf(w, "%v  received %d commits from %d contributors\n",
		name, x.Contributions, x.Contributors,
	)
	fmt.Fprintf(w, "commit window started  %s and ended %s \n",
		start.commit.Author.When.Format(ts),
		end.commit.Author.When.Format(ts),
	)
	fmt.Fprintf(w, "\n committers \n -----------\n")
	for _, c := range x.Committers {
		fmt.Fprintf(w, "- %v \n", c)
	}
}

func calc(x []*object.Commit) *info {
	m := make(map[string]struct{})
	var committers []string
	cm := make(map[string]struct{})
	for _, v := range x {
		m[v.Author.Email] = struct{}{}
		cm[v.Author.Name] = struct{}{}
	}
	for v := range cm {
		if !strings.Contains(v, " ") {
			continue
		}
		committers = append(committers, v)
	}
	sort.Strings(committers)
	return &info{
		Contributors:  len(m),
		Contributions: len(x),
		Committers:    committers,
	}
}
