// Copyright 2018 Hajime Hoshi
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package oggloop provides a function to get LOOPSTART and LOOPLENGTH information
// from a Ogg/Vorbis meta data as RPG Maker does.
package oggloop

import (
	"fmt"
	"io"
	"regexp"
	"strconv"
)

var (
	loopStartRe  = regexp.MustCompile(`LOOPSTART=([0-9]+)`)
	loopLengthRe = regexp.MustCompile(`LOOPLENGTH=([0-9]+)`)
)

type errReader struct {
	r   io.Reader
	err error
}

func (r *errReader) ReadByte() byte {
	return r.ReadBytes(1)[0]
}

func (r *errReader) ReadBytes(n int) []byte {
	buf := make([]byte, n)
	if r.err != nil {
		return buf
	}
	if n == 0 {
		return buf
	}
	if n < 0 {
		r.err = fmt.Errorf("oggloop: reading bytes should be positive: %d", n)
		return buf
	}
	if _, err := io.ReadFull(r.r, buf); err != nil {
		r.err = err
	}
	return buf
}

func (r *errReader) Skip(n int) {
	if r.err != nil {
		return
	}
	if n == 0 {
		return
	}
	if n < 0 {
		r.err = fmt.Errorf("oggloop: skipping bytes should be positive: %d", n)
		return
	}
	r.ReadBytes(n)
}

func mustAtoi(str string) int {
	n, err := strconv.Atoi(str)
	if err != nil {
		panic(err)
	}
	return n
}

// Read reads the given src as an Ogg/Vorbis stream and returns LOOPSTART and LOOPLENGTH meta data
// values. Read returns an error when IO error happens.
func Read(src io.Reader) (loopStart, loopLength int, err error) {
	r := &errReader{r: src}
	defer func() {
		if r.err != nil {
			err = r.err
		}
	}()

	for {
		if string(r.ReadBytes(4)) != "OggS" {
			break
		}
		r.Skip(26 - 4)
		headerFound := false
		nseg := r.ReadByte()
		segs := r.ReadBytes(int(nseg))

		for i := 0; i < len(segs); i++ {
			headerType := r.ReadByte()

			b := r.ReadBytes(4)

			if string(b) != "vorb" {
				r.Skip(int(segs[i])-5)
				continue
			}
			if headerType != 3 {
				r.Skip(int(segs[i])-5)
				headerFound = true
				continue
			}

			// If the segment size is 255, the segment content continues to its next.
			// https://www.xiph.org/ogg/doc/framing.html
			size := 0
			for ; i < len(segs); i++ {
				size += int(segs[i])
				if segs[i] < 255 {
					break
				}
			}
			size -= 5
			meta := r.ReadBytes(size)
			if m := loopStartRe.FindSubmatch(meta); len(m) > 1 {
				loopStart = mustAtoi(string(m[1]))
			}
			if m := loopLengthRe.FindSubmatch(meta); len(m) > 1 {
				loopLength = mustAtoi(string(m[1]))
			}
			headerFound = true
		}
		if !headerFound {
			break
		}
	}
	return
}
