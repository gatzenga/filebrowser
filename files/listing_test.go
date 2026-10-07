package files

import "testing"

func names(items []*FileInfo) []string {
	out := make([]string, len(items))
	for i, item := range items {
		out[i] = item.Name
	}
	return out
}

func TestSortByDuration(t *testing.T) {
	t.Parallel()

	newListing := func(asc bool) Listing {
		return Listing{
			Sorting: Sorting{By: "duration", Asc: asc},
			Items: []*FileInfo{
				{Name: "notes.txt"},
				{Name: "b-long.mkv", Duration: 7200},
				{Name: "a-short.mp4", Duration: 60},
				{Name: "c-medium.mkv", Duration: 1800},
				{Name: "also.txt"},
			},
		}
	}

	for name, tc := range map[string]struct {
		asc  bool
		want []string
	}{
		"ascending":  {asc: true, want: []string{"a-short.mp4", "c-medium.mkv", "b-long.mkv", "also.txt", "notes.txt"}},
		"descending": {asc: false, want: []string{"b-long.mkv", "c-medium.mkv", "a-short.mp4", "also.txt", "notes.txt"}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			l := newListing(tc.asc)
			l.ApplySort()

			got := names(l.Items)
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Fatalf("order = %v, want %v", got, tc.want)
				}
			}
		})
	}
}
