#!/bin/bash

set +u

echo "Finding PProf host:port"
HOSTS=$(lsof -c "${PACKAGE_EXECUTABLE:0:15}" | grep LISTEN | grep localhost | sed -E 's/rlp.*(localhost:\w+).*/\1/')

let HOST
for h in $HOSTS; do
    if curl --fail "$h/debug/pprof/" &> /dev/null 2> /dev/null; then
        HOST=$h
        break
    fi
done

TEMP_DIR=$(mktemp -d)
PROFILE_DIR="$TEMP_DIR/profiles"
mkdir "$PROFILE_DIR"


TARBALL_PATH="/tmp/$PACKAGE_EXECUTABLE-$(hostname)-profile-$(date --rfc-3339=date).tgz"

if [ "$HOST" = "" ]; then
    echo "Unable to find pprof"
    exit 1
fi

echo "PProf found at ${HOST}"
echo "Collecting profiles. This may take a while... "
curl "$HOST/debug/pprof/" > "$PROFILE_DIR/pprof.html" 2> /dev/null
curl "$HOST/debug/pprof/goroutine?debug=1" > "$PROFILE_DIR/goroutine.dump" 2> /dev/null
curl "$HOST/debug/pprof/heap?debug=1" > "$PROFILE_DIR/heap.dump" 2> /dev/null
curl "$HOST/debug/pprof/profile" > "$PROFILE_DIR/cpu.dump" 2> /dev/null
curl "$HOST/debug/pprof/trace?seconds=30" > "$PROFILE_DIR/trace.dump" 2> /dev/null
cp "$PACKAGE_DIR/$PACKAGE_EXECUTABLE" "$PROFILE_DIR/$PACKAGE_EXECUTABLE"

echo "Packaging profiles..."
pushd $TEMP_DIR > /dev/null
    tar czf "$TARBALL_PATH" profiles
popd > /dev/null

rm -rf "$TEMP_DIR"

echo "Profiles located at $TARBALL_PATH"
