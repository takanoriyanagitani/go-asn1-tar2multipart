#!/bin/sh

input=./sample.d/input.tar
output=./sample.d/output.asn1.multipart.dat

geninput(){
	echo generating input tar file...

	mkdir -p sample.d

	echo hw > ./sample.d/t1.txt
	echo hw2 > ./sample.d/t2.txt

	find \
		sample.d \
		-type f \
		-name '*.txt' |
		tar \
			--create \
			--verbose \
			--files-from /dev/stdin \
			--norecurse |
		dd \
			if=/dev/stdin \
			of="${input}" \
			bs=1048576 \
			status=none
}

test -f "${input}" || geninput

cat "${input}" |
	./asn1-tar2multipart |
	dd \
		if=/dev/stdin \
		of="${output}" \
		bs=1048576 \
		status=none

ls -l "${input}" "${output}"
