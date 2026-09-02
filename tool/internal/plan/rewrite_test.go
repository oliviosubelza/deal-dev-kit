package plan

import "testing"

var webRules = map[string]string{
	"@/components/ui/": "@/shared/ui/",
	"@/hooks/":         "@/shared/hooks/",
	"@/lib/":           "@/shared/lib/",
}

func TestRewriteImports(t *testing.T) {
	tests := []struct {
		name  string
		rules map[string]string
		in    string
		want  string
	}{
		{
			name:  "component import",
			rules: webRules,
			in:    `import { Button } from "@/components/ui/button"`,
			want:  `import { Button } from "@/shared/ui/button"`,
		},
		{
			name:  "lib import",
			rules: webRules,
			in:    `import { cn } from "@/lib/utils"`,
			want:  `import { cn } from "@/shared/lib/utils"`,
		},
		{
			name:  "single quotes",
			rules: webRules,
			in:    `import { Badge } from '@/components/ui/badge'`,
			want:  `import { Badge } from '@/shared/ui/badge'`,
		},
		{
			name:  "relative imports are left alone",
			rules: webRules,
			in:    `import type { FilterDef } from './filter-types'`,
			want:  `import type { FilterDef } from './filter-types'`,
		},
		{
			name:  "external packages are left alone",
			rules: webRules,
			in:    `import * as React from "react"`,
			want:  `import * as React from "react"`,
		},
		{
			name:  "longest prefix wins",
			rules: map[string]string{"@/components/": "@/x/", "@/components/ui/": "@/shared/ui/"},
			in:    `from "@/components/ui/button"`,
			want:  `from "@/shared/ui/button"`,
		},
		{
			name:  "prose outside quotes is untouched",
			rules: webRules,
			in:    "// Components live in @/components/ui/ by convention\n",
			want:  "// Components live in @/components/ui/ by convention\n",
		},
		{
			name:  "no rules is a passthrough",
			rules: nil,
			in:    `import { cn } from "@/lib/utils"`,
			want:  `import { cn } from "@/lib/utils"`,
		},
		{
			name:  "multiple imports in one file",
			rules: webRules,
			in: "import { cn } from \"@/lib/utils\"\n" +
				"import { Button } from \"@/components/ui/button\"\n" +
				"import { useIsMobile } from \"@/hooks/use-mobile\"\n",
			want: "import { cn } from \"@/shared/lib/utils\"\n" +
				"import { Button } from \"@/shared/ui/button\"\n" +
				"import { useIsMobile } from \"@/shared/hooks/use-mobile\"\n",
		},
		{
			name:  "unterminated quote does not lose content",
			rules: webRules,
			in:    `const broken = "@/lib/utils`,
			want:  `const broken = "@/lib/utils`,
		},
		{
			name:  "template literal content is preserved",
			rules: webRules,
			in:    "const c = `flex items-center`",
			want:  "const c = `flex items-center`",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(newRewriter(tt.rules).apply([]byte(tt.in)))
			if got != tt.want {
				t.Errorf("apply()\n got: %q\nwant: %q", got, tt.want)
			}
		})
	}
}

func TestRewriteLeavesBinaryContentAlone(t *testing.T) {
	binary := []byte{0xff, 0xfe, 0x00, 0x01, 0x80}
	if got := newRewriter(webRules).apply(binary); string(got) != string(binary) {
		t.Errorf("binary content was modified: % x", got)
	}
}

func TestRewriteIsDeterministic(t *testing.T) {
	in := []byte(`import { Button } from "@/components/ui/button"`)
	first := string(newRewriter(webRules).apply(in))
	for i := 0; i < 20; i++ {
		if got := string(newRewriter(webRules).apply(in)); got != first {
			t.Fatalf("map iteration order leaked: %q vs %q", got, first)
		}
	}
}
