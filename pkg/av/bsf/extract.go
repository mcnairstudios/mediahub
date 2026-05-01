package bsf

/*
#cgo pkg-config: libavcodec
#include <libavcodec/avcodec.h>
#include <libavcodec/bsf.h>
#include <libavcodec/packet.h>

static int get_new_extradata(AVPacket *pkt, uint8_t **data, int *size) {
    const AVPacketSideData *sd = av_packet_side_data_get(
        pkt->side_data, pkt->side_data_elems,
        AV_PKT_DATA_NEW_EXTRADATA
    );
    if (!sd) return -1;
    *data = sd->data;
    *size = (int)sd->size;
    return 0;
}
*/
import "C"

import (
	"errors"
	"fmt"
	"unsafe"

	"github.com/asticode/go-astiav"
)

type ExtraDataExtractor struct {
	bsfCtx *astiav.BitStreamFilterContext
	outPkt *astiav.Packet
}

func NewExtraDataExtractor(codecID astiav.CodecID, timeBase astiav.Rational) (*ExtraDataExtractor, error) {
	bsf := astiav.FindBitStreamFilterByName("extract_extradata")
	if bsf == nil {
		return nil, errors.New("bsf: extract_extradata filter not found")
	}

	bsfCtx, err := astiav.AllocBitStreamFilterContext(bsf)
	if err != nil {
		return nil, fmt.Errorf("bsf: alloc context: %w", err)
	}

	bsfCtx.InputCodecParameters().SetCodecID(codecID)
	bsfCtx.SetInputTimeBase(timeBase)

	if err := bsfCtx.Initialize(); err != nil {
		bsfCtx.Free()
		return nil, fmt.Errorf("bsf: initialize: %w", err)
	}

	outPkt := astiav.AllocPacket()
	if outPkt == nil {
		bsfCtx.Free()
		return nil, errors.New("bsf: failed to allocate output packet")
	}

	return &ExtraDataExtractor{
		bsfCtx: bsfCtx,
		outPkt: outPkt,
	}, nil
}

func (e *ExtraDataExtractor) ProcessPacket(pkt *astiav.Packet) ([]byte, error) {
	if err := e.bsfCtx.SendPacket(pkt); err != nil {
		if errors.Is(err, astiav.ErrEagain) {
			return nil, nil
		}
		return nil, fmt.Errorf("bsf: send packet: %w", err)
	}

	for {
		e.outPkt.Unref()
		if err := e.bsfCtx.ReceivePacket(e.outPkt); err != nil {
			if errors.Is(err, astiav.ErrEagain) || errors.Is(err, astiav.ErrEof) {
				return nil, nil
			}
			return nil, fmt.Errorf("bsf: receive packet: %w", err)
		}

		ed := extractNewExtradata(e.outPkt)
		if ed != nil {
			return ed, nil
		}
	}
}

func (e *ExtraDataExtractor) Close() {
	if e.outPkt != nil {
		e.outPkt.Free()
		e.outPkt = nil
	}
	if e.bsfCtx != nil {
		e.bsfCtx.Free()
		e.bsfCtx = nil
	}
}

func extractNewExtradata(pkt *astiav.Packet) []byte {
	type packetLayout struct {
		c *C.AVPacket
	}
	cp := (*packetLayout)(unsafe.Pointer(pkt))
	if cp.c == nil {
		return nil
	}

	var data *C.uint8_t
	var size C.int
	if C.get_new_extradata(cp.c, &data, &size) != 0 {
		return nil
	}
	if size <= 0 || data == nil {
		return nil
	}

	out := make([]byte, int(size))
	copy(out, unsafe.Slice((*byte)(unsafe.Pointer(data)), int(size)))
	return out
}
