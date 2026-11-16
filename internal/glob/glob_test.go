package glob

import "testing"

func TestMatch(t *testing.T) {
	tests := []struct {
		pattern string
		text    string
		want    bool
	}{
		{"*", "", true},
		{"*", "anything", true},
		{"cd *", "cd /tmp", true},
		{"cd *", "echo cd /tmp", false},
		{"*--token*", "gh auth --token=abc", true},
		{"*token=*", "export github_token=ghp_x", true},
		{"a?c", "abc", true},
		{"a?c", "ac", false},
		{"[a-c]x", "bx", true},
		{"[a-c]x", "dx", false},
		{"[^a-c]x", "dx", true},
		{"[^a-c]x", "ax", false},
		{"[]]", "]", true},
		{"[a-]", "-", true},
		{"[abc", "a", false},
		{"*://*:*@*", "git clone https://user:pw@host/repo", true},
		{"*://*:*@*", "git clone https://host/repo", false},
		{"mysql* -p[^ ]*", "mysql -u root -psecret db", true},
		{"mysql* -p[^ ]*", "mysql -u root -p db", false},
		{"*\u00e4*", "gr\u00e4tz", true},
	}

	for _, test := range tests {
		if got := Match(test.pattern, test.text); got != test.want {
			t.Errorf("Match(%q, %q) = %v, want %v", test.pattern, test.text, got, test.want)
		}
	}
}
