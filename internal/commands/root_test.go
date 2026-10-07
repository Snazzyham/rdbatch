package commands

import (
	"reflect"
	"testing"

	"github.com/soham/rdbatch/internal/api"
)

func TestDownloadableFileIDs(t *testing.T) {
	files := []api.File{{ID: "1", Name: "movie.mkv"}, {ID: "2", Name: "folder/source.nfo"}, {ID: "3", Name: "SOURCE.NFO"}, {ID: "4", Name: "subtitles.srt"}, {ID: "5", Name: "notes.nfo.txt"}}
	for _, tt := range []struct {
		name           string
		selected, want []string
	}{
		{name: "whole torrent", want: []string{"1", "4", "5"}},
		{name: "explicit selection", selected: []string{"1", "2", "3"}, want: []string{"1"}},
		{name: "only nfo selected", selected: []string{"2", "3"}},
		{name: "unknown selection", selected: []string{"missing"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := downloadableFileIDs(files, tt.selected); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}
