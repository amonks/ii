package agent

import "testing"

func TestJSONLTailDiff(t *testing.T) {
	cases := []struct {
		name string
		prev string
		curr string
		want string
	}{
		{
			name: "initial prints full when complete",
			prev: "",
			curr: "{\"a\":1}\n",
			want: "{\"a\":1}\n",
		},
		{
			name: "initial prints nothing when incomplete",
			prev: "",
			curr: "{\"a\":1}",
			want: "",
		},
		{
			name: "append prints only new lines when complete",
			prev: "{\"a\":1}\n",
			curr: "{\"a\":1}\n{\"b\":2}\n",
			want: "{\"b\":2}\n",
		},
		{
			name: "append prints nothing when trailing line incomplete",
			prev: "{\"a\":1}\n",
			curr: "{\"a\":1}\n{\"b\":2}",
			want: "",
		},
		{
			name: "non-append fallback prints only complete lines",
			prev: "{\"a\":1}\n",
			curr: "{\"X\":9}\n{\"b\":2}",
			want: "{\"X\":9}\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := JSONLTailDiff(tc.prev, tc.curr); got != tc.want {
				t.Fatalf("diff mismatch\nprev=%q\ncurr=%q\n got=%q\nwant=%q", tc.prev, tc.curr, got, tc.want)
			}
		})
	}
}
