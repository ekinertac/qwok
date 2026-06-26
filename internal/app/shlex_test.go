package app

import "testing"

func TestSplitCommand(t *testing.T) {
	cases := []struct {
		in   string
		want []string
		err  bool
	}{
		{"npm run dev", []string{"npm", "run", "dev"}, false},
		{"  pnpm   dev ", []string{"pnpm", "dev"}, false},
		{`python manage.py runserver`, []string{"python", "manage.py", "runserver"}, false},
		{`go run "./cmd/my app"`, []string{"go", "run", "./cmd/my app"}, false},
		{`echo 'single quoted'`, []string{"echo", "single quoted"}, false},
		{`bad "unterminated`, nil, true},
		{"   ", nil, true},
	}
	for _, c := range cases {
		got, err := splitCommand(c.in)
		if c.err {
			if err == nil {
				t.Errorf("splitCommand(%q): expected error, got %v", c.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("splitCommand(%q): unexpected error %v", c.in, err)
			continue
		}
		if len(got) != len(c.want) {
			t.Errorf("splitCommand(%q) = %v, want %v", c.in, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("splitCommand(%q)[%d] = %q, want %q", c.in, i, got[i], c.want[i])
			}
		}
	}
}
