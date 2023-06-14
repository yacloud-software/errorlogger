/*
help to store blocks of arbitrary length in streams. 8-bit clean
*/
package streamblock

import (
	"fmt"
	"io"
)

const (
	ESCAPE_BYTE         = 0xFF
	START_BYTE          = 0x01
	END_BYTE            = 0x00
	ESCAPED_START_BYTE  = 0x02
	ESCAPED_END_BYTE    = 0x03
	ESCAPED_ESCAPE_BYTE = 0x04
)

// write in blocks
type blockWriter struct {
	w io.Writer
}

// return a block writer that writes to "w"
func NewBlockWriter(w io.Writer) io.Writer {
	res := &blockWriter{w: w}
	return res
}

// escape and write a block to disk
func (b *blockWriter) Write(block []byte) (int, error) {
	wr := make([]byte, len(block))
	copy(wr, block)
	for i := len(block) - 1; i >= 0; i-- {
		if block[i] == START_BYTE || block[i] == END_BYTE || block[i] == ESCAPE_BYTE {
			wr = append(wr, 0)
			copy(wr[i+1:], wr[i:])
			wr[i] = ESCAPE_BYTE
			if wr[i+1] == START_BYTE {
				wr[i+1] = ESCAPED_START_BYTE
			} else if wr[i+1] == END_BYTE {
				wr[i+1] = ESCAPED_END_BYTE
			} else if wr[i+1] == ESCAPE_BYTE {
				wr[i+1] = ESCAPED_ESCAPE_BYTE
			}
		}
	}
	wr = append([]byte{START_BYTE}, wr...) // prefix the start-of-block marker with 1
	wr = append(wr, END_BYTE)              // append the end-of-block marker with 0
	n, err := b.w.Write(wr)
	return n, err
}

// read in blocks
type BlockReader struct {
	r            io.Reader
	rs           io.ReadSeeker
	buf          []byte
	bytes_in_buf int
	read_index   int
	seekable     bool
}

func NewBlockReader(r io.Reader) *BlockReader {
	res := &BlockReader{r: r, buf: make([]byte, 8192)}
	return res
}
func NewSeekableBlockReader(r io.ReadSeeker) *BlockReader {
	res := &BlockReader{
		seekable: true,
		r:        r,
		rs:       r,
		buf:      make([]byte, 8192)}
	return res
}

// reads one block and returns it unescaped.position pointer at beginning of next block
func (b *BlockReader) ReadBlock() ([]byte, error) {
	// find block start, marked by unescaped 1
	for {
		nb, err := b.nextByte()
		if err != nil {
			return nil, err
		}
		if nb == START_BYTE {
			break
		}
	}
	// rest follows is a block until unescaped 0
	var res []byte
	for {
		nb, err := b.nextByte()
		if err != nil {
			return res, err
		}
		if nb == END_BYTE {
			break
		}
		res = append(res, nb)
	}
	return unescape_block(res), nil
}
func (b *BlockReader) nextByte() (byte, error) {
get_byte:
	if b.bytes_in_buf > 0 {
		res := b.buf[b.read_index]
		b.read_index++
		b.bytes_in_buf--
		return res, nil
	}

	n, err := b.r.Read(b.buf)
	if err != nil {
		return 0, err
	}
	if n == 0 {
		return 0, fmt.Errorf("BlockReader read 0 bytes. blocking reader required")
	}
	b.bytes_in_buf = n
	b.read_index = 0
	goto get_byte
}

// position pointer at beginning of block n from end of file
func (br *BlockReader) SeekFromEnd(n int) error {
	return br.SeekFromEndForBlocks(n, func(b []byte) bool { return true })
}

// position pointer at beginning of block n from end of file, counting only blocks that match f()
func (br *BlockReader) SeekFromEndForBlocks(n int, f func(b []byte) bool) error {
	if !br.seekable {
		return fmt.Errorf("this blockreader is not seekable")
	}
	if n < 0 {
		n = 0 - n
	}
	//	fmt.Printf("Seeking to %d\n", n)
	_, err := br.rs.Seek(-1, io.SeekEnd) // position at end
	if err != nil {
		return err
	}
	packets_skipped := 0
	for {
		b, err := br.ReadPreviousBlock()
		if err != nil {
			return err
		}
		if f(b) {
			packets_skipped++
		}
		if packets_skipped >= n {
			return nil
		}

	}
}

// read the block that _ends_ before or at the current position. position the seek pointer at the beginnning-1 of the block
func (br *BlockReader) ReadPreviousBlock() ([]byte, error) {
	var cur_block []byte
	// find end of block
	for {
		b, err := br.prevByte()
		if err != nil {
			return nil, err
		}
		if b == END_BYTE {
			break
		}
	}

	// find start of block
	for {
		b, err := br.prevByte()
		if err != nil {
			return nil, err
		}
		if b == START_BYTE {
			break
		}
		cur_block = append([]byte{b}, cur_block...)
	}
	return unescape_block(cur_block), nil
}

// given data as read, return as user expects it
func unescape_block(read []byte) []byte {
	res := make([]byte, len(read))
	escaped := false
	n := 0
	for _, b := range read {
		if b == ESCAPE_BYTE {
			escaped = true
			continue
		}
		if escaped {
			b = escaped_byte_to_normal(b)
			escaped = false
		}
		res[n] = b
		n++

	}
	return res[:n]
}

func escaped_byte_to_normal(b byte) byte {
	if b == ESCAPED_START_BYTE {
		return START_BYTE
	} else if b == ESCAPED_END_BYTE {
		return END_BYTE
	} else if b == ESCAPED_ESCAPE_BYTE {
		return ESCAPE_BYTE
	}
	fmt.Printf("Invalid escaped byte 0x%02X\n", b)
	return b
}

// seek to end and readpreviousblock
func (br *BlockReader) ReadLastBlock() ([]byte, error) {
	_, err := br.rs.Seek(-1, io.SeekEnd) // position at end
	if err != nil {
		return nil, err
	}
	return br.ReadPreviousBlock()
}

// read a byte and position pointer at the byte BEFORE the one read
func (b *BlockReader) prevByte() (byte, error) {
	// test, most inefficient ever implementation
	buf := make([]byte, 1)
	_, err := b.rs.Read(buf)
	if err != nil {
		return 0, err
	}
	_, err = b.rs.Seek(-2, io.SeekCurrent)
	if err != nil {
		return 0, err
	}
	return buf[0], nil
}
func hexstr(a []byte) string {
	s := ""
	deli := ""
	for _, b := range a {
		s = s + deli + fmt.Sprintf("%02X", b)
		deli = " "
	}
	return s
}
