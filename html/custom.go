package html

import (
	"errors"
	"fmt"
	"io"

	a "golang.org/x/net/html/atom"
)

func parseWithIndexes(p *parser) (map[*Node][2]int, error) {
	// Iterate until EOF. Any other error will cause an early return.
	var err error
	var globalBufDif int
	var prevEndBuf int
	var tokenIndex [2]int
	tokenMap := make(map[*Node][2]int)
	for err != io.EOF {
		// CDATA sections are allowed only in foreign content.
		n := p.oe.top()
		p.tokenizer.AllowCDATA(n != nil && n.Namespace != "")

		t := p.top().FirstChild
		for {
			if t != nil && t.NextSibling != nil {
				t = t.NextSibling
			} else {
				break
			}
		}
		tokenMap[t] = tokenIndex
		if prevEndBuf > p.tokenizer.data.end {
			globalBufDif += prevEndBuf
		}
		prevEndBuf = p.tokenizer.data.end
		// Read and parse the next token.
		p.tokenizer.Next()
		start := p.tokenizer.data.start + globalBufDif
		// If the node was already found, it means that we're looking at the closing tag.
		// If so, the start position should come from the opening tag.
		if _, ok := tokenMap[t]; ok {
			start = tokenMap[t][0]
		}

		tokenIndex = [2]int{start, p.tokenizer.data.end + globalBufDif}

		p.tok = p.tokenizer.Token()
		if p.tok.Type == ErrorToken {
			err = p.tokenizer.Err()
			if err != nil && err != io.EOF {
				return tokenMap, err
			}
		}
		p.parseCurrentToken()
	}
	return tokenMap, nil
}

// ParseFragmentWithIndexes parses a fragment of HTML and returns the nodes
// that were found. If the fragment is the InnerHTML for an existing element,
// pass that element in context.
func ParseFragmentWithIndexes(r io.Reader, context *Node) ([]*Node, map[*Node][2]int, error) {
	contextTag := ""
	if context != nil {
		if context.Type != ElementNode {
			return nil, nil, errors.New("html: ParseFragment of non-element Node")
		}
		// The next check isn't just context.DataAtom.String() == context.Data because
		// it is valid to pass an element whose tag isn't a known atom. For example,
		// DataAtom == 0 and Data = "tagfromthefuture" is perfectly consistent.
		if context.DataAtom != a.Lookup([]byte(context.Data)) {
			return nil, nil, fmt.Errorf("html: inconsistent Node: DataAtom=%q, Data=%q", context.DataAtom, context.Data)
		}
		contextTag = context.DataAtom.String()
	}
	p := &parser{
		tokenizer: NewTokenizerFragment(r, contextTag),
		doc: &Node{
			Type: DocumentNode,
		},
		scripting: true,
		fragment:  true,
		context:   context,
	}

	root := &Node{
		Type:     ElementNode,
		DataAtom: a.Html,
		Data:     a.Html.String(),
	}
	p.doc.AppendChild(root)
	p.oe = nodeStack{root}
	p.resetInsertionMode()

	for n := context; n != nil; n = n.Parent {
		if n.Type == ElementNode && n.DataAtom == a.Form {
			p.form = n
			break
		}
	}

	tokenMap, err := parseWithIndexes(p)
	if err != nil {
		return nil, nil, err
	}

	parent := p.doc
	if context != nil {
		parent = root
	}

	var result []*Node
	for c := parent.FirstChild; c != nil; {
		next := c.NextSibling
		parent.RemoveChild(c)
		result = append(result, c)
		c = next
	}
	return result, tokenMap, nil
}
