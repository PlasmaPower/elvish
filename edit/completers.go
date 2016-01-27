package edit

import (
	"fmt"
	"io/ioutil"
	"path"
	"strings"

	"github.com/elves/elvish/eval"
	"github.com/elves/elvish/parse"
)

// A completer takes the current node
type completer func(parse.Node, *Editor) []*candidate

var completers = []struct {
	name string
	completer
}{
	{"variable", complVariable},
	{"command name", complNewForm},
	{"command name", makeCompoundCompleter(complFormHead)},
	{"argument", complNewArg},
	{"argument", makeCompoundCompleter(complArg)},
}

func complVariable(n parse.Node, ed *Editor) []*candidate {
	primary, ok := n.(*parse.Primary)
	if !ok || primary.Type != parse.Variable {
		return nil
	}

	head := primary.Value[1:]
	cands := []*candidate{}
	for variable := range ed.evaler.Global() {
		if strings.HasPrefix(variable, head) {
			cands = append(cands, &candidate{
				source: styled{variable[len(head):], attrForType[Variable]},
				menu:   styled{"$" + variable, attrForType[Variable]}})
		}
	}
	return cands
}

func complNewForm(n parse.Node, ed *Editor) []*candidate {
	if _, ok := n.(*parse.Chunk); ok {
		return complFormHeadInner("", ed)
	}
	if _, ok := n.Parent().(*parse.Chunk); ok {
		return complFormHeadInner("", ed)
	}
	return nil
}

func makeCompoundCompleter(
	f func(*parse.Compound, string, *Editor) []*candidate) completer {
	return func(n parse.Node, ed *Editor) []*candidate {
		pn, ok := n.(*parse.Primary)
		if !ok {
			return nil
		}
		cn, head := simpleCompound(pn)
		if cn == nil {
			return nil
		}
		return f(cn, head, ed)
	}
}

func complFormHead(cn *parse.Compound, head string, ed *Editor) []*candidate {
	if isFormHead(cn) {
		return complFormHeadInner(head, ed)
	}
	return nil
}

func complFormHeadInner(head string, ed *Editor) []*candidate {
	cands := []*candidate{}
	foundCommand := func(s string) {
		if strings.HasPrefix(s, head) {
			cands = append(cands, &candidate{
				source: styled{s[len(head):], styleForGoodCommand},
				menu:   styled{s, ""},
			})
		}
	}
	for special := range isBuiltinSpecial {
		foundCommand(special)
	}
	for variable := range ed.evaler.Global() {
		if strings.HasPrefix(variable, eval.FnPrefix) {
			foundCommand(variable[3:])
		}
	}
	for command := range ed.isExternal {
		foundCommand(command)
	}
	return cands
}

func complNewArg(n parse.Node, ed *Editor) []*candidate {
	sn, ok := n.(*parse.Sep)
	if !ok {
		return nil
	}
	if _, ok := sn.Parent().(*parse.Form); !ok {
		return nil
	}
	return complArgInner("", ed)
}

func complArg(cn *parse.Compound, head string, ed *Editor) []*candidate {
	return complArgInner(head, ed)
}

func complArgInner(head string, ed *Editor) []*candidate {
	// Assume that the argument is an incomplete filename
	dir, file := path.Split(head)
	if dir == "" {
		dir = "."
	}
	names, err := fileNames(dir)
	cands := []*candidate{}

	if err != nil {
		ed.pushTip(fmt.Sprintf("cannot list directory %s: %v", dir, err))
		return cands
	}

	// Make candidates out of elements that match the file component.
	for s := range names {
		if strings.HasPrefix(s, file) {
			cands = append(cands, &candidate{
				source: styled{s[len(file):], ""},
				menu:   styled{s, defaultLsColor.determineAttr(s)},
			})
		}
	}

	return cands
}

func fileNames(dir string) (<-chan string, error) {
	infos, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	names := make(chan string, 32)
	go func() {
		for _, info := range infos {
			names <- info.Name()
		}
		close(names)
	}()
	return names, nil
}