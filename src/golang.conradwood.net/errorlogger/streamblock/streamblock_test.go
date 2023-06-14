package streamblock

import (
	"bytes"
	"testing"
)

func FuzzStreamBlock(f *testing.F) {
	for _, seed := range [][]byte{
		{},
		{0, 0, 0, 0xFF, 0, 0},
		{9},
		{0xa},
		{0xff},
		{1, 2, 3, 4},
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, in []byte) {
		t.Logf("Fuzz: %v\n", in)
		checkSame(t, in)
	})
}

func TestPositioning(t *testing.T) {
	checkposread(t, deterministic1)
}

func deterministic1(n int) []byte {
	size := 30
	res := make([]byte, size)
	for i := 0; i < size; i++ {
		res[i] = byte(i + n)
	}
	l := size / 2
	res[l] = 0
	l++
	res[l] = 1
	l++
	n1 := byte(n >> 8)
	n2 := byte(n)
	res = append([]byte{n1, n2}, res...)
	return res
}

func TestPacketFromEnd(t *testing.T) {
	check_packet_read_from_end(t, deterministic1)
}

func TestReadLastPacket(t *testing.T) {
	f := deterministic1
	z, err := write_blocks(50, f)
	if err != nil {
		t.Errorf("failed to write: %s", err)
		return
	}
	nr := NewSeekableBlockReader(bytes.NewReader(z))
	got, err := nr.ReadLastBlock()
	if err != nil {
		t.Errorf("failed to write: %s", err)
		return
	}
	expect := f(49)
	if !issame(got, expect) {
		t.Errorf("Mismatch: expected\n\"%s\", but got\n\"%s\"\n", hexstr(expect), hexstr(got))
	}

}

// return the bytes actually written
func write_blocks(count int, f func(n int) []byte) ([]byte, error) {
	out := &bytes.Buffer{}
	nw := NewBlockWriter(out)
	for i := 0; i < count; i++ {
		d := f(i)
		_, err := nw.Write(d)
		if err != nil {
			return nil, err
		}
	}
	return out.Bytes(), nil
}

func check_packet_read_from_end(t *testing.T, f func(b int) []byte) {
	search_for := func(d []byte) bool {
		if d[4] == 0x51 {
			return true
		}
		return false
	}
	max := 9999
	fromEnd := -1
	out := &bytes.Buffer{}
	nw := NewBlockWriter(out)
	var r [][]byte // counting the packets that we will retrieve
	for i := 1; i < max; i++ {
		d := f(i)
		_, err := nw.Write(d)
		if err != nil {
			t.Errorf("failed to write: %s", err)
			return
		}
		if search_for(d) {
			r = append(r, d)
		}
	}
	z := out.Bytes()
	nr := NewSeekableBlockReader(bytes.NewReader(z))
	nr.SeekFromEndForBlocks(fromEnd, search_for)
	got, err := nr.ReadBlock()
	if err != nil {
		t.Errorf("failed to read: %s", err)
		return
	}
	expect := r[len(r)+fromEnd]
	if !issame(got, expect) {
		t.Errorf("Mismatch: At position %d from end (absolute %d), expected\n\"%s\", but got\n\"%s\"\n", fromEnd, max+fromEnd, hexstr(expect), hexstr(got))
	}

}

// give a function that gives a deterministic set of bytes B for a packet N
func checkposread(t *testing.T, f func(b int) []byte) {
	max := 99
	fromEnd := -5
	out := &bytes.Buffer{}
	nw := NewBlockWriter(out)
	for i := 1; i < max; i++ {
		_, err := nw.Write(f(i))
		if err != nil {
			t.Errorf("failed to write: %s", err)
			return
		}
	}
	z := out.Bytes()
	nr := NewSeekableBlockReader(bytes.NewReader(z))
	nr.SeekFromEnd(fromEnd)
	got, err := nr.ReadBlock()
	if err != nil {
		t.Errorf("failed to read: %s", err)
		return
	}
	expect := f(max + fromEnd)
	if !issame(got, expect) {
		t.Errorf("Mismatch: At position %d from end (absolute %d), expected\n\"%s\", but got\n\"%s\"\n", fromEnd, max+fromEnd, hexstr(expect), hexstr(got))
	}
}

func checkSame(t *testing.T, z []byte) {
	t.Logf("checksame: "+hexstr(z)+" %v", z)
	out := &bytes.Buffer{}
	NewBlockWriter(out).Write(z)

	a := out.Bytes()
	out = &bytes.Buffer{}
	out.Write(a)

	x, err := NewBlockReader(out).ReadBlock()
	if err != nil {
		t.Errorf("For \"%s\" (%s), Failed to re-read: %s", hexstr(z), hexstr(x), err)
		return
	}

	if !issame(x, z) {
		t.Errorf("For \"%s\" (%s), re-reading got \"%s\"", hexstr(z), hexstr(a), hexstr(x))
		return
	}
}
func issame(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i, _ := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
