#!/bin/bash

# benchmark output lines look like:
# total requests = 743
# total duration = 389.41
# requests/sec = 1.91
# mean = 524ms, stddev = 88ms
# fastest: 27ms, slowest 725ms
fieldscript='
/^total requests/ {treqs = $4}
/^mean/ {
    mean = $3
    sub("ms,", "", mean)
    stddev = $6
    sub("ms", "", stddev)
}
/^fastest/ {
    min = $2
    sub("ms,", "", min)
    max = $4
    sub("ms", "", max)
}
END {print treqs "," mean "," min "," max "," stddev}'

echo applications,seconds,mongo-total,mongo-mean,mongo-min,mongo-max,mongo-stddev,raft-total,raft-mean,raft-min,raft-max,raft-stddev
for units in $(seq 1 40); do
    raft=$(./leadership-benchmark -t 20 --units $units raft.yaml --raft | awk "$fieldscript")
    orig=$(./leadership-benchmark -t 20 --units $units mongo.yaml | awk "$fieldscript")
    echo $units,20,$orig,$raft
done
