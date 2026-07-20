package handlers

import (
	"strings"
	"testing"
)

func TestEdgeNodeRuntimeScopesIncludeWebSocketScope(t *testing.T) {
	for _, scope := range []string{"edge:register", "edge:printer", "edge:heartbeat"} {
		if !strings.Contains(" "+edgeNodeRuntimeScopes+" ", " "+scope+" ") {
			t.Fatalf("edge node activation scopes must include %q: %q", scope, edgeNodeRuntimeScopes)
		}
	}
}
