package netlink

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"os"
	"syscall"
	"unsafe"
)

type tcMsg struct {
	tcm_family  uint8
	tcm_pad1    uint8
	tcm_pad2    uint16
	tcm_ifindex int32
	tcm_handle  uint32
	tcm_parent  uint32
	tcm_info    uint32
}

type QDisc struct {
	handle  uint32
	refCnt  uint32
	ifIndex int
	ifName  string
	name    string
	parent  string
	options interface{}
}

const SizeofTcMsg = 0x14

const (
	TCA_UNSPEC = iota
	TCA_KIND
	TCA_OPTIONS
	TCA_STATS
	TCA_XSTATS
	TCA_RATE
	TCA_FCNT
	TCA_STATS2
	TCA_STAB
	__TCA_MAX
)

const TCA_MAX = __TCA_MAX - 1

const (
	TC_H_UNSPEC  = 0x00000000
	TC_H_ROOT    = 0xFFFFFFFF
	TC_H_INGRESS = 0xFFFFFFF1
)

const TIME_UNITS_PER_SEC = 1000000

var clock_factor float64
var tick_in_usec float64

func init() {
	var clock_res, t2us, us2t uint32

	fp, err := os.Open("/proc/net/psched")
	if err != nil {
		return
	}
	defer fp.Close()

	n, err := fmt.Fscanf(fp, "%08x %08x %08x", &t2us, &us2t, &clock_res)
	if err != nil && n != 3 {
		return
	}

	if clock_res == 1000000000 {
		t2us = us2t
	}

	clock_factor = float64(clock_res) / TIME_UNITS_PER_SEC
	tick_in_usec = float64(t2us) / float64(us2t) * clock_factor
}

func netlinkRouteAttrAndValue(b []byte) (*syscall.RtAttr, []byte, int, error) {
	a := (*syscall.RtAttr)(unsafe.Pointer(&b[0]))
	if int(a.Len) < syscall.SizeofRtAttr || int(a.Len) > len(b) {
		return nil, nil, 0, syscall.EINVAL
	}
	return a, b[syscall.SizeofRtAttr:], rtaAlignOf(int(a.Len)), nil
}

func parseRtAttr(b []byte) ([]syscall.NetlinkRouteAttr, error) {
	var attrs []syscall.NetlinkRouteAttr
	for len(b) >= syscall.SizeofRtAttr {
		a, vbuf, alen, err := netlinkRouteAttrAndValue(b)
		if err != nil {
			return nil, err
		}
		ra := syscall.NetlinkRouteAttr{Attr: *a, Value: vbuf[:int(a.Len)-syscall.SizeofRtAttr]}
		attrs = append(attrs, ra)
		b = b[alen:]
	}
	return attrs, nil
}

func getHandleById(handle uint32) string {
	switch {
	case handle == TC_H_ROOT:
		return "root"
	case handle == TC_H_UNSPEC:
		return "none"
	case handle&0xFFFF0000 == 0:
		return fmt.Sprintf(":%x", handle&0x0000FFFF)
	case handle&0x0000FFFF == 0:
		return fmt.Sprintf("%x:", (handle&0xFFFF0000)>>16)
	default:
		return fmt.Sprintf("%x:%x", (handle&0xFFFF0000)>>16, handle&0x0000FFFF)
	}
}

func GetNlMessages(iface *net.Interface, proto, flags int) ([]syscall.NetlinkMessage, error) {
	s, err := getNetlinkSocket()
	if err != nil {
		return nil, err
	}
	defer s.Close()

	wb := newNetlinkRequest(proto, flags)

	msg := newIfInfomsg(syscall.AF_UNSPEC)
	msg.Index = int32(iface.Index)
	wb.AddData(msg)

	if err := s.Send(wb); err != nil {
		return nil, err
	}

	pid, err := s.GetPid()
	if err != nil {
		return nil, err
	}

outer:
	for {
		msgs, err := s.Receive()
		if err != nil {
			return nil, err
		}
		for _, m := range msgs {
			if err := s.CheckMessage(m, wb.Seq, pid); err != nil {
				if err == io.EOF {
					break outer
				}
				return nil, err
			}
		}
		return msgs, nil
	}

	return nil, errors.New("unknown error")
}

func GetQdisc(iface *net.Interface) ([]QDisc, error) {
	msgs, err := GetNlMessages(iface, syscall.RTM_GETQDISC, syscall.NLM_F_DUMP)
	if err != nil {
		return nil, err
	}

	qDiscs := make([]QDisc, 0)

	for _, m := range msgs {
		if m.Header.Type != syscall.RTM_NEWQDISC && m.Header.Type != syscall.RTM_DELQDISC {
			continue
		}

		dataLen := int(m.Header.Len - syscall.SizeofNlMsghdr)
		if dataLen < 0 || dataLen != len(m.Data) {
			fmt.Printf("incorrect nlmsg payload\n")
			return nil, errors.New("incorrect nlmsg payload")
		}

		msg := (*tcMsg)(unsafe.Pointer(&m.Data[0:SizeofTcMsg][0]))
		if msg.tcm_ifindex != int32(iface.Index) {
			continue
		}

		tcMsgPayload := m.Data[SizeofTcMsg:]
		rtAttrs, err := parseRtAttr(tcMsgPayload)
		if err != nil {
			fmt.Printf("error parsing rtattr\n")
			return nil, err
		}

		var qDisc QDisc
		qDisc.handle = msg.tcm_handle >> 16
		qDisc.refCnt = msg.tcm_info
		qDisc.ifIndex = int(msg.tcm_ifindex)
		intf, err := net.InterfaceByIndex(qDisc.ifIndex)
		if err != nil {
			fmt.Printf("Unable to get device\n")
			return nil, errors.New("unable to get device")
		}
		qDisc.ifName = intf.Name
		if msg.tcm_parent == TC_H_ROOT {
			qDisc.parent = "root"
		} else if msg.tcm_parent != 0 {
			qDisc.parent = "parent " + getHandleById(msg.tcm_parent)
		}

		for _, rtAttr := range rtAttrs {

			switch rtAttr.Attr.Type {
			case TCA_KIND:
				if rtAttr.Attr.Len == 0 || string(rtAttr.Value) == "" {
					fmt.Printf("Null Kind\n")
					return nil, errors.New("null kind")
				}

				qDisc.name = string(rtAttr.Value[:bytes.Index(rtAttr.Value, []byte{0})])

			case TCA_OPTIONS:
				switch qDisc.name {
				case "tbf":
					opts, err := getTbfOpts(rtAttr.Value)
					if err != nil {
						fmt.Printf("error parsing options\n")
						return nil, err
					}
					qDisc.options = opts

				case "ingress":
					qDisc.options = string("---------------- ")

				default:
					qDisc.options = string("[cannot parse qdisc parameters]")
				}

			case TCA_STATS, TCA_STATS2, TCA_XSTATS:

			default:
			}
		}

		qDiscs = append(qDiscs, qDisc)
	}

	return qDiscs, nil
}

func printRate(rate uint32) {
	tmp := float64(rate) * 8

	switch {
	case tmp >= 1000.0*1000000000.0:
		fmt.Printf("%.0fGbit ", tmp/1000000000.0)
	case tmp >= 1000.0*1000000.0:
		fmt.Printf("%.0fMbit ", tmp/1000000.0)
	case tmp >= 1000.0*1000.0:
		fmt.Printf("%.0fKbit ", tmp/1000.0)
	default:
		fmt.Printf("%.0fbit ", tmp)
	}
}

func printSize(sz uint32) {
	tmp := float64(sz)

	switch {
	case sz >= 1024*1024 && math.Abs(float64(1024*1024*int(tmp/(1024*1024))-int(sz))) < 1024:
		fmt.Printf("%dMb ", int(tmp/(1024*1024)))
	case sz >= 1024 && math.Abs(float64(1024*int(tmp/1024)-int(sz))) < 16:
		fmt.Printf("%dKb ", int(tmp/1024))
	default:
		fmt.Printf("%db ", sz)
	}
}

func printTime(time uint32) {
	tmp := float64(time)

	switch {

	case tmp >= TIME_UNITS_PER_SEC:
		fmt.Printf("%.1fs ", tmp/TIME_UNITS_PER_SEC)
	case tmp >= TIME_UNITS_PER_SEC/1000:
		fmt.Printf("%.1fms ", tmp/(TIME_UNITS_PER_SEC/1000))
	default:
		fmt.Printf("%f", time)
	}
}

func tick2Time(ticks uint32) float64 {
	return float64(ticks) / tick_in_usec
}

func PrintQDisc(intf string) error {

	iface, err := net.InterfaceByName(intf)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Non-existent interface: %s", intf)
		return err
	}

	qDiscs, err := GetQdisc(iface)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting QDiscs: %s", err)
		return err
	}

	for _, qDisc := range qDiscs {
		fmt.Printf("qdisc %s %x: %s ", qDisc.name, qDisc.handle, qDisc.parent)
		if qDisc.refCnt != 1 {
			fmt.Printf("refcnt %d ", qDisc.refCnt)
		}
		switch qDisc.name {
		case "tbf":
			tbufQDisc := qDisc.options.(*TcTbfQopt)

			fmt.Printf("rate ")
			printRate(tbufQDisc.rate.rate)

			fmt.Printf("burst ")
			printSize(uint32(float64(tbufQDisc.rate.rate) * tick2Time(tbufQDisc.buffer) / TIME_UNITS_PER_SEC))

			fmt.Printf("lat ")
			latency := TIME_UNITS_PER_SEC*(float64(tbufQDisc.limit)/float64(tbufQDisc.rate.rate)) - tick2Time(tbufQDisc.buffer)
			if tbufQDisc.peakrate.rate != 0 {
				lat2 := TIME_UNITS_PER_SEC*(float64(tbufQDisc.limit)/float64(tbufQDisc.peakrate.rate)) - tick2Time(tbufQDisc.mtu)
				if lat2 > latency {
					latency = lat2
				}
			}
			printTime(uint32(latency))

		case "ingress":
			options, _ := qDisc.options.(string)
			fmt.Printf("%s", options)

		default:
			options, _ := qDisc.options.(string)
			fmt.Printf("%s", options)
		}

		fmt.Printf("\n")
	}

	return nil
}
