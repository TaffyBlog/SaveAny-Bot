package handlers

import (
	"testing"

	"github.com/gotd/td/telegram/downloader"
	"github.com/gotd/td/tg"
	"github.com/krau/SaveAny-Bot/pkg/tfile"
)

type fakeAlbumFile struct {
	id   int
	size int64
	msg  *tg.Message
}

func (f *fakeAlbumFile) Location() tg.InputFileLocationClass { return nil }
func (f *fakeAlbumFile) Dler() downloader.Client             { return nil }
func (f *fakeAlbumFile) Size() int64                         { return f.size }
func (f *fakeAlbumFile) Name() string                        { return "" }
func (f *fakeAlbumFile) SetName(string)                      {}
func (f *fakeAlbumFile) Message() *tg.Message                { return f.msg }

func videoMessage(id int, size int64) *tg.Message {
	doc := &tg.Document{
		ID:       int64(id),
		MimeType: "video/mp4",
		Size:     size,
		Attributes: []tg.DocumentAttributeClass{
			&tg.DocumentAttributeVideo{},
		},
	}
	return &tg.Message{
		ID:    id,
		Media: &tg.MessageMediaDocument{Document: doc},
	}
}

func photoMessage(id int) *tg.Message {
	return &tg.Message{
		ID:    id,
		Media: &tg.MessageMediaPhoto{Photo: &tg.PhotoEmpty{ID: int64(id)}},
	}
}

func buildAlbum() []tfile.TGFileMessage {
	return []tfile.TGFileMessage{
		&fakeAlbumFile{id: 1, size: 1000, msg: photoMessage(1)},
		&fakeAlbumFile{id: 2, size: 5000, msg: videoMessage(2, 5000)},
		&fakeAlbumFile{id: 3, size: 9000, msg: videoMessage(3, 9000)},
		&fakeAlbumFile{id: 4, size: 800, msg: photoMessage(4)},
	}
}

func idsOf(files []tfile.TGFileMessage) []int {
	out := make([]int, 0, len(files))
	for _, f := range files {
		out = append(out, f.Message().GetID())
	}
	return out
}

func TestFilterAlbumItemsByMode(t *testing.T) {
	album := buildAlbum()
	cases := []struct {
		name string
		mode string
		want []int
	}{
		{"batch keeps everything", albumModeBatch, []int{1, 2, 3, 4}},
		{"unknown mode falls back to batch", "whatever", []int{1, 2, 3, 4}},
		{"first keeps only the first item", albumModeFirst, []int{1}},
		{"video keeps only videos", albumModeVideo, []int{2, 3}},
		{"largest keeps only the biggest item", albumModeLargest, []int{3}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := idsOf(filterAlbumItemsByMode(album, c.mode))
			if len(got) != len(c.want) {
				t.Fatalf("got %v, want %v", got, c.want)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Fatalf("got %v, want %v", got, c.want)
				}
			}
		})
	}
}

func TestFilterAlbumItemsKeepsOriginalSlice(t *testing.T) {
	album := buildAlbum()
	// Filtering must not reorder or mutate the caller's slice.
	_ = filterAlbumItemsByMode(album, albumModeLargest)
	if got := idsOf(album); len(got) != 4 || got[0] != 1 || got[3] != 4 {
		t.Fatalf("album was mutated: %v", got)
	}
}
