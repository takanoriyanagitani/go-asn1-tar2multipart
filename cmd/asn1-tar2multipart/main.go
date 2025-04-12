package main

import (
	"context"
	"fmt"
	"iter"
	"log"
	"os"
	"strconv"

	tm "github.com/takanoriyanagitani/go-asn1-tar2multipart"
	. "github.com/takanoriyanagitani/go-asn1-tar2multipart/util"
)

var envValByKey func(string) IO[string] = Lift(
	func(key string) (string, error) {
		val, found := os.LookupEnv(key)
		switch found {
		case true:
			return val, nil
		default:
			return "", fmt.Errorf("env var %s missing", key)
		}
	},
)

var rdr tm.TarReader = tm.TarReaderFromStdin()

var str2int64 func(string) (int64, error) = ComposeErr(
	strconv.Atoi,
	func(i int) (int64, error) { return int64(i), nil },
)

var tarItemSizeLimit IO[int64] = Bind(
	envValByKey("ENV_TAR_ITEM_SIZE_LIMIT"),
	Lift(str2int64),
).Or(Of(int64(1048576)))

var items IO[iter.Seq2[tm.TarItemAsn1, error]] = Bind(
	tarItemSizeLimit,
	Lift(func(limit int64) (iter.Seq2[tm.TarItemAsn1, error], error) {
		return rdr.ToItems(limit), nil
	}),
)

var items2stdout func(iter.Seq2[tm.TarItemAsn1, error]) IO[Void] = func(
	items iter.Seq2[tm.TarItemAsn1, error],
) IO[Void] {
	return func(ctx context.Context) (Void, error) {
		return Empty, tm.ItemsToStdout(ctx, items)
	}
}

var stdin2tar2der2mpart2stdout IO[Void] = Bind(
	items,
	items2stdout,
)

func main() {
	_, e := stdin2tar2der2mpart2stdout(context.Background())
	if nil != e {
		log.Printf("%v\n", e)
	}
}
