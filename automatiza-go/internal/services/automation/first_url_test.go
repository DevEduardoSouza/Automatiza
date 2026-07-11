package automation

import "testing"

// TestFirstURL locks the concatenated-URL salvage: users paste several share
// links into one list item, and before this fix the mangled string failed
// Shopee enrichment ("invalid shopee URL after expansion") and the item was
// burned as a dead link. firstURL must recover the first link — and must NOT
// touch a legitimate single URL.
func TestFirstURL(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			"single url untouched",
			"https://s.shopee.com.br/4fu22BiyBK",
			"https://s.shopee.com.br/4fu22BiyBK",
		},
		{
			"real prod corruption: three links glued",
			"https://s.shopee.com.br/9pcWOYfvethttps://s.shopee.com.br/3qLCdVkuUrhttps://s.shopee.com.br/9Uzg04dtjZ",
			"https://s.shopee.com.br/9pcWOYfvet",
		},
		{
			"two links, second is plain http",
			"https://s.shopee.com.br/abchttp://example.com/x",
			"https://s.shopee.com.br/abc",
		},
		{
			"bare 'http' in query does not cut",
			"https://shopee.com.br/p?utm_source=http",
			"https://shopee.com.br/p?utm_source=http",
		},
		{
			"expanded url with params untouched",
			"https://shopee.com.br/opaanlp/457864097/25191982379?__mobile__=1&exp_group=rollout",
			"https://shopee.com.br/opaanlp/457864097/25191982379?__mobile__=1&exp_group=rollout",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := firstURL(tc.in); got != tc.want {
				t.Fatalf("firstURL(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
