package integrationtest

import (
	"bytes"
	"context"
	"errors"
	"io"
	"math"
	"testing"
	"time"

	bootstrap2 "github.com/ipfs/go-ipfs/core/bootstrap"
	"github.com/ipfs/go-ipfs/core/coreapi"
	"github.com/ipfs/go-ipfs/thirdparty/unit"

	files "github.com/ipfs/go-ipfs-files"
	pstore "github.com/libp2p/go-libp2p-peerstore"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	testutil "github.com/libp2p/go-testutil"
)

func TestThreeLeggedCatTransfer(t *testing.T) {
	conf := testutil.LatencyConfig{
		NetworkLatency:    0,
		RoutingLatency:    0,
		BlockstoreLatency: 0,
	}
	if err := RunThreeLeggedCat(RandomBytes(100*unit.MB), conf); err != nil {
		t.Fatal(err)
	}
}

func TestThreeLeggedCatDegenerateSlowBlockstore(t *testing.T) {
	SkipUnlessEpic(t)
	conf := testutil.LatencyConfig{BlockstoreLatency: 50 * time.Millisecond}
	if err := RunThreeLeggedCat(RandomBytes(1*unit.KB), conf); err != nil {
		t.Fatal(err)
	}
}

func TestThreeLeggedCatDegenerateSlowNetwork(t *testing.T) {
	SkipUnlessEpic(t)
	conf := testutil.LatencyConfig{NetworkLatency: 400 * time.Millisecond}
	if err := RunThreeLeggedCat(RandomBytes(1*unit.KB), conf); err != nil {
		t.Fatal(err)
	}
}

func TestThreeLeggedCatDegenerateSlowRouting(t *testing.T) {
	SkipUnlessEpic(t)
	conf := testutil.LatencyConfig{RoutingLatency: 400 * time.Millisecond}
	if err := RunThreeLeggedCat(RandomBytes(1*unit.KB), conf); err != nil {
		t.Fatal(err)
	}
}

func TestThreeLeggedCat100MBMacbookCoastToCoast(t *testing.T) {
	SkipUnlessEpic(t)
	conf := testutil.LatencyConfig{}.NetworkNYtoSF().BlockstoreSlowSSD2014().RoutingSlow()
	if err := RunThreeLeggedCat(RandomBytes(100*unit.MB), conf); err != nil {
		t.Fatal(err)
	}
}

func RunThreeLeggedCat(data []byte, conf testutil.LatencyConfig) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// create network
	mn := mocknet.New(ctx)
	mn.SetLinkDefaults(mocknet.LinkOptions{
		Latency: conf.NetworkLatency,
		// TODO add to conf. This is tricky because we want 0 values to be functional.
		Bandwidth: math.MaxInt32,
	})

	bootstrapApi, err := coreapi.New(
		coreapi.Ctx(ctx),

		coreapi.Online(),
		coreapi.MockHost(mn),
	)
	if err != nil {
		return err
	}
	bootstrap := bootstrapApi.Node()
	defer bootstrap.Close()

	adderApi, err := coreapi.New(
		coreapi.Ctx(ctx),

		coreapi.Online(),
		coreapi.MockHost(mn),
	)
	if err != nil {
		return err
	}
	adder := adderApi.Node()
	defer adder.Close()

	catterApi, err := coreapi.New(
		coreapi.Ctx(ctx),

		coreapi.Online(),
		coreapi.MockHost(mn),
	)
	if err != nil {
		return err
	}
	catter := catterApi.Node()
	defer catter.Close()

	err = mn.LinkAll()
	if err != nil {
		return err
	}

	bis := bootstrap.Peerstore.PeerInfo(bootstrap.PeerHost.ID())
	bcfg := bootstrap2.BootstrapConfigWithPeers([]pstore.PeerInfo{bis})
	if err := adder.Bootstrap(bcfg); err != nil {
		return err
	}
	if err := catter.Bootstrap(bcfg); err != nil {
		return err
	}

	added, err := adderApi.Unixfs().Add(ctx, files.NewBytesFile(data))
	if err != nil {
		return err
	}

	readerCatted, err := catterApi.Unixfs().Get(ctx, added)
	if err != nil {
		return err
	}

	// verify
	var bufout bytes.Buffer
	_, err = io.Copy(&bufout, readerCatted.(io.Reader))
	if err != nil {
		return err
	}
	if !bytes.Equal(bufout.Bytes(), data) {
		return errors.New("catted data does not match added data")
	}
	return nil
}
