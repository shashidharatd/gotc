package netlink

import (
	"errors"
	"fmt"
	"unsafe"
)

const (
	TCA_TBF_UNSPEC = iota
	TCA_TBF_PARMS
	TCA_TBF_RTAB
	TCA_TBF_PTAB
	__TCA_TBF_MAX
)

type TcRateSpec struct {
	cell_log   uint8
	linklayer  uint8 /* lower 4 bits */
	overhead   uint16
	cell_align uint16
	mpu        uint16
	rate       uint32
}

type TcTbfQopt struct {
	rate     TcRateSpec
	peakrate TcRateSpec
	limit    uint32
	buffer   uint32
	mtu      uint32
}

func getTbfOpts(b []byte) (*TcTbfQopt, error) {

	optRtAttrs, err := parseRtAttr(b)
	if err != nil {
		fmt.Printf("error parsing tc options\n")
		return nil, err
	}

	for _, optRtAttr := range optRtAttrs {
		if optRtAttr.Attr.Type == TCA_TBF_PARMS {
			opts := (*TcTbfQopt)(unsafe.Pointer(&optRtAttr.Value[0]))
			return opts, nil
		}
	}

	return nil, errors.New("tbf: unable to parse options")
}
