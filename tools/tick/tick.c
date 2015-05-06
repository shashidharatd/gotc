#include <stdio.h>

int main()
{
	FILE *fp;
	unsigned int clock_res;
	unsigned int t2us;
	unsigned int us2t;
        double clock_factor=0.0, tick_in_usec=0.0;

	fp = fopen("/proc/net/psched", "r");
	if (fp == NULL)
		return -1;

	if (fscanf(fp, "%08x%08x%08x", &t2us, &us2t, &clock_res) != 3) {
		fclose(fp);
		return -1;
	}
	fclose(fp);

	/* compatibility hack: for old iproute binaries (ignoring
	 * the kernel clock resolution) the kernel advertises a
	 * tick multiplier of 1000 in case of nano-second resolution,
	 * which really is 1. */
	if (clock_res == 1000000000)
		t2us = us2t;

	clock_factor  = (double)clock_res / 1000000;
	tick_in_usec = (double)t2us / us2t * clock_factor;

        printf("tick_in_usec=%f,clock_factor=%f,t2us=%d,us2t=%d,clock_res=%d\n", tick_in_usec,clock_factor,t2us,us2t,clock_res);        
}
