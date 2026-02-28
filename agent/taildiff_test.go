package agent_test

import (
	"testing"

	"github.com/amonks/incrementum/agent"
)

func TestTranscriptTailDiff(t *testing.T) {
	cases := []struct {
		name string
		prev string
		curr string
		want string
	}{
		{
			name: "initial prints full",
			prev: "",
			curr: "hello\n",
			want: "hello\n",
		},
		{
			name: "prefix prints only appended",
			prev: "hello\n",
			curr: "hello\nworld\n",
			want: "world\n",
		},
		{
			name: "non-prefix prints full snapshot",
			prev: "hello\nworld\n",
			curr: "HELLO\nWORLD\n",
			want: "HELLO\nWORLD\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := agent.TranscriptTailDiff(tc.prev, tc.curr); got != tc.want {
				t.Fatalf("diff mismatch\nprev=%q\ncurr=%q\n got=%q\nwant=%q", tc.prev, tc.curr, got, tc.want)
			}
		})
	}
}
