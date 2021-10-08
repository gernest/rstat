package main

import (
	"bufio"
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
	var from []*object.Commit
	err = object.NewCommitPreorderIter(start.commit, nil, nil).ForEach(func(c *object.Commit) error {
		from = append(from, c)
		return nil
	})

	var to []*object.Commit
	err = object.NewCommitPreorderIter(end.commit, nil, nil).ForEach(func(c *object.Commit) error {
		to = append(to, c)
		return nil
	})
	if err != nil {
		if !errors.Is(err, io.EOF) {
			return err
		}
	}
	format(w, start.name, merge(from, to), end, start)
	return nil
}

type info struct {
	Contributors  int
	Contributions int
	Committers    []*Committer
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
		fmt.Fprintf(w, "- %v \n", c.name)
		for _, msg := range c.commits {
			fmt.Fprintf(w, "    - %v \n", firstLine(msg.Message))
		}
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "## Changelog\n%s..%s\n", start.name, end.name)
	commitSummary(w, commits)
}

type Committer struct {
	name    string
	commits []*object.Commit
}

func calc(x []*object.Commit) *info {
	m := make(map[string]struct{})
	var committers []*Committer
	cm := make(map[string]*Committer)
	for _, v := range x {
		m[v.Author.Email] = struct{}{}
		if n, ok := cm[v.Author.Name]; ok {
			n.commits = append(n.commits, v)
		} else {
			cm[v.Author.Name] = &Committer{
				name:    v.Author.Name,
				commits: []*object.Commit{v},
			}
		}
	}
	for _, v := range cm {
		if !strings.Contains(v.name, " ") {
			continue
		}
		committers = append(committers, v)
	}
	sort.Slice(committers, func(i, j int) bool {
		return committers[i].name < committers[j].name
	})
	return &info{
		Contributors:  len(m),
		Contributions: len(x),
		Committers:    committers,
	}
}

func commitSummary(w io.Writer, commits []*object.Commit) {
	for _, commit := range commits {
		fmt.Fprintf(w, "%s %s\n", short(commit.Hash), firstLine(commit.Message))
	}
}

func short(p plumbing.Hash) string {
	return p.String()[:7]
}

func firstLine(msg string) string {
	scan := bufio.NewScanner(strings.NewReader(msg))
	scan.Split(bufio.ScanLines)
	for scan.Scan() {
		return scan.Text()
	}
	return ""
}

func merge(a, b []*object.Commit) (o []*object.Commit) {
	seen := make(map[string]struct{})
	for _, v := range b {
		seen[v.Hash.String()] = struct{}{}
	}
	for _, v := range a {
		_, ok := seen[v.Hash.String()]
		if !ok {
			o = append(o, v)
		}
	}
	return
}
