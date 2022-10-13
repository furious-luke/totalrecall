#!/bin/bash

# go build -buildmode=c-archive -o libmain.a main.go
# gcc -I `pg_config --includedir-server` -o ./test worker.c ./totalrecall_worker.so
# gcc -shared -fPIC -o totalrecall_worker.so -I `pg_config --includedir-server` worker.c libmain.a
# CGO_LDFLAGS="-g -shared" CGO_CFLAGS="-g -I `pg_config --includedir-server`" go build -buildmode=c-shared -gcflags=all="-N -l" -o totalrecall_worker.so
gcc -shared -fPIC -I `pg_config --includedir-server` -o totalrecall_worker.so worker.c

# if [ ! $(which pg_config) ]; then
# 	echo "ERROR: pg_config not found"
# 	exit 1
# fi

# INCLUDEDIR=$(pg_config --includedir-server)
# LIBDIR=$(pg_config --pkglibdir)
# LIBNAME="totalrecall_worker.so"
# LIBOUTPUT="${LIBNAME}"

# export CGO_CFLAGS="-I ${INCLUDEDIR}"
# export CGO_LDFLAGS="-shared"

# echo "Building ${LIBOUTPUT}"
# go build -buildmode=c-shared -o ${LIBOUTPUT}
# chmod a+x ${LIBOUTPUT}
