package glance

import (
	"reflect"
	"sort"
	"testing"
)

func TestGroupDockerContainerChildrenCategoryFilter(t *testing.T) {
	container := func(name string, labels dockerContainerLabels) dockerContainerJsonResponse {
		return dockerContainerJsonResponse{Names: []string{"/" + name}, Labels: labels}
	}

	containers := []dockerContainerJsonResponse{
		container("gitea", dockerContainerLabels{
			dockerContainerLabelID:       "gitea",
			dockerContainerLabelCategory: "external",
		}),
		container("gitea-db", dockerContainerLabels{
			dockerContainerLabelParent: "gitea",
		}),
		container("gitea-hidden", dockerContainerLabels{
			dockerContainerLabelParent: "gitea",
			dockerContainerLabelHide:   "true",
		}),
		container("grafana", dockerContainerLabels{
			dockerContainerLabelCategory: "internal",
		}),
		container("uncategorized", nil),
	}

	tests := []struct {
		name     string
		category string
		expected map[string][]string
	}{
		{
			name:     "no category filter includes all containers",
			category: "",
			expected: map[string][]string{
				"gitea":         {"gitea-db"},
				"grafana":       nil,
				"uncategorized": nil,
			},
		},
		{
			name:     "child without category is included via matching parent",
			category: "external",
			expected: map[string][]string{
				"gitea": {"gitea-db"},
			},
		},
		{
			name:     "child is excluded when parent does not match",
			category: "internal",
			expected: map[string][]string{
				"grafana": nil,
			},
		},
		{
			name:     "no containers match",
			category: "nonexistent",
			expected: map[string][]string{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parents, children := groupDockerContainerChildren(containers, false, test.category)

			// Mirrors how fetchDockerContainers attaches children to their parents
			actual := make(map[string][]string, len(parents))
			for i := range parents {
				parent := &parents[i]
				var childNames []string

				if id := parent.Labels.getOrDefault(dockerContainerLabelID, ""); id != "" {
					for j := range children[id] {
						childNames = append(childNames, deriveDockerContainerName(&children[id][j], false))
					}
				}

				sort.Strings(childNames)
				actual[deriveDockerContainerName(parent, false)] = childNames
			}

			if !reflect.DeepEqual(actual, test.expected) {
				t.Errorf("expected %v, got %v", test.expected, actual)
			}
		})
	}
}
