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

func skip(src io.Reader, n int) error {
	if n == 0 {
		return nil
	}
	if n < 0 {
		return fmt.Errorf("oggloop: skipping byte should be positive: %d", n)
	}
	if _, err := io.ReadFull(src, make([]byte, n)); err != nil {
		return err
	}
	return nil
}

func readByte(src io.Reader) (byte, error) {
	b := make([]byte, 1)
	if _, err := io.ReadFull(src, b); err != nil {
		return 0, err
	}
	return b[0], nil
}

var (
	loopStartRe  = regexp.MustCompile(`LOOPSTART=([0-9]+)`)
	loopLengthRe = regexp.MustCompile(`LOOPLENGTH=([0-9]+)`)
)

// Read reads the given src as an Ogg/Vorbis stream and returns LOOPSTART and LOOPLENGTH meta data
// values. Read returns an error when IO error happens.
func Read(src io.Reader) (loopStart, loopLength int, err error) {
	for {
		b := make([]byte, 4)
		if _, err := io.ReadFull(src, b); err != nil {
			return 0, 0, err
		}
		if string(b) != "OggS" {
			break
		}

		if err := skip(src, 26-4); err != nil {
			return 0, 0, err
		}

		headerFound := false
		nseg, err := readByte(src)
		if err != nil {
			return 0, 0, err
		}
		segs := make([]byte, nseg)
		if _, err := io.ReadFull(src, segs); err != nil {
			return 0, 0, err
		}

		for i := 0; i < len(segs); i++ {
			headerType, err := readByte(src)
			if err != nil {
				return 0, 0, err
			}

			b := make([]byte, 4)
			if _, err := io.ReadFull(src, b); err != nil {
				return 0, 0, err
			}

			if string(b) != "vorb" {
				if err := skip(src, int(segs[i])-5); err != nil {
					return 0, 0, err
				}
				continue
			}
			if headerType != 3 {
				if err := skip(src, int(segs[i])-5); err != nil {
					return 0, 0, err
				}
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
			meta := make([]byte, size)
			if _, err := io.ReadFull(src, meta); err != nil {
				return 0, 0, err
			}
			if m := loopStartRe.FindSubmatch(meta); len(m) > 1 {
				n, err := strconv.Atoi(string(m[1]))
				if err != nil {
					panic(err)
				}
				loopStart = n
			}
			if m := loopLengthRe.FindSubmatch(meta); len(m) > 1 {
				n, err := strconv.Atoi(string(m[1]))
				if err != nil {
					panic(err)
				}
				loopLength = n
			}
			headerFound = true
		}
		if !headerFound {
			break
		}
	}
	return
}
