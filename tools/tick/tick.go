package main

import (
	"fmt"
	"os"
)

func main() {

	var clock_res, t2us, us2t uint32

	fp, err := os.Open("/proc/net/psched")
	if err != nil {
		return
	}
	defer fp.Close()

	n, err := fmt.Fscanf(fp, "%08x%08x%08x", &t2us, &us2t, &clock_res)
	if err != nil && n != 3 {
		return
	}

	if clock_res == 1000000000 {
		t2us = us2t
	}

	clock_factor := float64(clock_res) / 1000000
	tick_in_usec := float64(t2us) / float64(us2t) * clock_factor

	fmt.Printf("tick_in_usec=%f, clock_factor=%f, t2us=%d, us2t=%d, clock_res=%d\n", tick_in_usec, clock_factor, t2us, us2t, clock_res)

}
