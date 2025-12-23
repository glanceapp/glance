package glance

import (
	"strings"

	"golang.org/x/net/html"
)

type searchableNode html.Node

func (n *searchableNode) findFirst(tag string, attrs ...string) *searchableNode {
	if tag == "" || n == nil {
		return nil
	}

	if len(attrs)%2 != 0 {
		panic("attributes must be in key-value pairs")
	}

	if n.matches(tag, attrs...) {
		return n
	}

	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if child.Type != html.ElementNode {
			continue
		}

		if found := (*searchableNode)(child).findFirst(tag, attrs...); found != nil {
			return found
		}
	}

	return nil
}

func (n *searchableNode) nthChild(index int) *searchableNode {
	if n == nil || index < 0 {
		return nil
	}

	if index == 0 {
		return n
	}

	count := 0
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if child.Type != html.ElementNode {
			continue
		}

		count++
		if count == index {
			return (*searchableNode)(child)
		}
	}

	return nil
}

func (n *searchableNode) findFirstChild(tag string, attrs ...string) *searchableNode {
	if tag == "" || n == nil {
		return nil
	}

	if len(attrs)%2 != 0 {
		panic("attributes must be in key-value pairs")
	}

	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if child.Type != html.ElementNode {
			continue
		}

		if child.Type == html.ElementNode && (*searchableNode)(child).matches(tag, attrs...) {
			return (*searchableNode)(child)
		}
	}

	return nil
}

func (n *searchableNode) findAll(tag string, attrs ...string) []*searchableNode {
	if tag == "" || n == nil {
		return nil
	}

	if len(attrs)%2 != 0 {
		panic("attributes must be in key-value pairs")
	}

	var results []*searchableNode

	if n.matches(tag, attrs...) {
		results = append(results, n)
	}

	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if child.Type != html.ElementNode {
			continue
		}

		results = append(results, (*searchableNode)(child).findAll(tag, attrs...)...)
	}

	return results
}

func (n *searchableNode) matches(tag string, attrs ...string) bool {
	if tag == "" || n == nil {
		return false
	}

	if len(attrs)%2 != 0 {
		panic("attributes must be in key-value pairs")
	}

	if n.Data != tag {
		return false
	}

	for i := 0; i < len(attrs); i += 2 {
		key := attrs[i]
		value := attrs[i+1]
		found := false
		for _, attr := range n.Attr {
			if attr.Key == key && attr.Val == value {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

func (n *searchableNode) text() string {
	if n == nil {
		return ""
	}

	if n.Type == html.TextNode {
		return strings.TrimSpace(n.Data)
	}

	var builder strings.Builder

	for child := n.FirstChild; child != nil; child = child.NextSibling {
		switch child.Type {
		case html.TextNode:
			builder.WriteString(strings.TrimSpace(child.Data))
		case html.ElementNode:
			builder.WriteString((*searchableNode)(child).text())
		}
	}

	return builder.String()
}
