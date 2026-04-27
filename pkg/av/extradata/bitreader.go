package extradata

import "fmt"

type bitReader struct {
	data []byte
	pos  int
}

func newBitReader(data []byte) *bitReader {
	return &bitReader{data: data}
}

func (r *bitReader) readBits(n int) (uint32, error) {
	if n < 0 || n > 32 {
		return 0, fmt.Errorf("bitReader: invalid bit count %d", n)
	}
	if n == 0 {
		return 0, nil
	}
	if (r.pos + n) > len(r.data)*8 {
		return 0, fmt.Errorf("bitReader: read past end of data (pos=%d, need=%d, have=%d bits)", r.pos, n, len(r.data)*8)
	}

	var val uint32
	for i := 0; i < n; i++ {
		byteIdx := r.pos >> 3
		bitIdx := 7 - (r.pos & 7)
		val = (val << 1) | uint32((r.data[byteIdx]>>bitIdx)&1)
		r.pos++
	}
	return val, nil
}

func (r *bitReader) readBit() (uint8, error) {
	v, err := r.readBits(1)
	return uint8(v), err
}

func (r *bitReader) readUE() (uint32, error) {
	leadingZeros := 0
	for {
		bit, err := r.readBit()
		if err != nil {
			return 0, err
		}
		if bit == 1 {
			break
		}
		leadingZeros++
		if leadingZeros > 31 {
			return 0, fmt.Errorf("bitReader: exp-golomb leading zeros exceed 31")
		}
	}
	if leadingZeros == 0 {
		return 0, nil
	}
	suffix, err := r.readBits(leadingZeros)
	if err != nil {
		return 0, err
	}
	return (1 << leadingZeros) - 1 + suffix, nil
}

func (r *bitReader) readSE() (int32, error) {
	k, err := r.readUE()
	if err != nil {
		return 0, err
	}
	if k%2 == 0 {
		return -int32(k / 2), nil
	}
	return int32((k + 1) / 2), nil
}

func (r *bitReader) skip(n int) {
	r.pos += n
}
