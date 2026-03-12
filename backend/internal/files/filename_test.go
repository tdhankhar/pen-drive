package files

import "testing"

func TestSanitizeFilename(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name         string
		input        string
		wantStored   string
		wantOriginal string
		wantErr      bool
	}{
		{name: "trims surrounding whitespace", input: " report.pdf ", wantOriginal: " report.pdf ", wantStored: "report.pdf"},
		{name: "collapses repeated spaces", input: "my   report.pdf", wantOriginal: "my   report.pdf", wantStored: "my report.pdf"},
		{name: "normalizes tabs and newlines", input: "a\tb\nc.txt", wantOriginal: "a\tb\nc.txt", wantStored: "a b c.txt"},
		{name: "drops traversal segments", input: "../../secret.txt", wantOriginal: "../../secret.txt", wantStored: "secret.txt"},
		{name: "replaces path separators with path flattening", input: "dir/name.txt", wantOriginal: "dir/name.txt", wantStored: "dir-name.txt"},
		{name: "leading dots are stripped", input: "..hidden", wantOriginal: "..hidden", wantStored: "hidden"},
		{name: "multi part extension preserved", input: "archive.tar.gz", wantOriginal: "archive.tar.gz", wantStored: "archive.tar.gz"},
		{name: "trailing dots removed", input: "photo.", wantOriginal: "photo.", wantStored: "photo"},
		{name: "hidden file becomes visible", input: ".gitignore", wantOriginal: ".gitignore", wantStored: "gitignore"},
		{name: "degenerate dots rejected", input: "...", wantErr: true},
		{name: "empty rejected", input: "", wantErr: true},
		{name: "whitespace rejected", input: "   ", wantErr: true},
		{name: "unicode preserved", input: "résumé.pdf", wantOriginal: "résumé.pdf", wantStored: "résumé.pdf"},
		{name: "unicode non latin preserved", input: "नमस्ते.txt", wantOriginal: "नमस्ते.txt", wantStored: "नमस्ते.txt"},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := SanitizeFilename(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got.Original != tc.wantOriginal {
				t.Fatalf("expected original %q, got %q", tc.wantOriginal, got.Original)
			}

			if got.Stored != tc.wantStored {
				t.Fatalf("expected stored %q, got %q", tc.wantStored, got.Stored)
			}

			gotAgain, err := SanitizeFilename(tc.input)
			if err != nil {
				t.Fatalf("unexpected error on second sanitize: %v", err)
			}
			if gotAgain != got {
				t.Fatalf("expected deterministic result, got %+v then %+v", got, gotAgain)
			}
		})
	}
}
