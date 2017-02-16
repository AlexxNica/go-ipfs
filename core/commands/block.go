package commands

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"github.com/ipfs/go-ipfs-cmds"
	"github.com/ipfs/go-ipfs-cmds/cmdsutil"
	"github.com/ipfs/go-ipfs/blocks"
	util "github.com/ipfs/go-ipfs/blocks/blockstore/util"

	mh "gx/ipfs/QmYDds3421prZgqKbLpEK7T9Aa2eVdQ7o3YarX1LVLdP2J/go-multihash"
	//u "gx/ipfs/Qmb912gdngC1UWwTkhuW8knyRbcWeu5kqkxBpveLmW8bSr/go-ipfs-util"
	cid "gx/ipfs/QmcTcsTvfaeEBRFo1TkFgT8sRmgi1n1LTZpecfVP8fzpGD/go-cid"
)

type BlockStat struct {
	Key  string
	Size int
}

func (bs BlockStat) String() string {
	return fmt.Sprintf("Key: %s\nSize: %d\n", bs.Key, bs.Size)
}

var BlockCmd = &cmds.Command{
	Helptext: cmdsutil.HelpText{
		Tagline: "Interact with raw IPFS blocks.",
		ShortDescription: `
'ipfs block' is a plumbing command used to manipulate raw IPFS blocks.
Reads from stdin or writes to stdout, and <key> is a base58 encoded
multihash.
`,
	},

	Subcommands: map[string]*cmds.Command{
		"stat": blockStatCmd,
		"get":  blockGetCmd,
		"put":  blockPutCmd,
		"rm":   blockRmCmd,
	},
}

var blockStatCmd = &cmds.Command{
	Helptext: cmdsutil.HelpText{
		Tagline: "Print information of a raw IPFS block.",
		ShortDescription: `
'ipfs block stat' is a plumbing command for retrieving information
on raw IPFS blocks. It outputs the following to stdout:

	Key  - the base58 encoded multihash
	Size - the size of the block in bytes

`,
	},

	Arguments: []cmdsutil.Argument{
		cmdsutil.StringArg("key", true, false, "The base58 multihash of an existing block to stat.").EnableStdin(),
	},
	Run: func(req cmds.Request, re cmds.ResponseEmitter) {
		b, err := getBlockForKey(req, req.Arguments()[0])
		if err != nil {
			re.SetError(err, cmdsutil.ErrNormal)
			return
		}

		re.Emit(&BlockStat{
			Key:  b.Cid().String(),
			Size: len(b.RawData()),
		})
	},
	Type: BlockStat{},
}

var blockGetCmd = &cmds.Command{
	Helptext: cmdsutil.HelpText{
		Tagline: "Get a raw IPFS block.",
		ShortDescription: `
'ipfs block get' is a plumbing command for retrieving raw IPFS blocks.
It outputs to stdout, and <key> is a base58 encoded multihash.
`,
	},

	Arguments: []cmdsutil.Argument{
		cmdsutil.StringArg("key", true, false, "The base58 multihash of an existing block to get.").EnableStdin(),
	},
	Run: func(req cmds.Request, re cmds.ResponseEmitter) {
		b, err := getBlockForKey(req, req.Arguments()[0])
		if err != nil {
			re.SetError(err, cmdsutil.ErrNormal)
			return
		}

		re.Emit(bytes.NewReader(b.RawData()))
	},
}

var blockPutCmd = &cmds.Command{
	Helptext: cmdsutil.HelpText{
		Tagline: "Store input as an IPFS block.",
		ShortDescription: `
'ipfs block put' is a plumbing command for storing raw IPFS blocks.
It reads from stdin, and <key> is a base58 encoded multihash.
`,
	},

	Arguments: []cmdsutil.Argument{
		cmdsutil.FileArg("data", true, false, "The data to be stored as an IPFS block.").EnableStdin(),
	},
	Options: []cmdsutil.Option{
		cmdsutil.StringOption("format", "f", "cid format for blocks to be created with.").Default("v0"),
		cmdsutil.StringOption("mhtype", "multihash hash function").Default("sha2-256"),
		cmdsutil.IntOption("mhlen", "multihash hash length").Default(-1),
	},
	Run: func(req cmds.Request, re cmds.ResponseEmitter) {
		n, err := req.InvocContext().GetNode()
		if err != nil {
			re.SetError(err, cmdsutil.ErrNormal)
			return
		}

		file, err := req.Files().NextFile()
		if err != nil {
			re.SetError(err, cmdsutil.ErrNormal)
			return
		}

		data, err := ioutil.ReadAll(file)
		if err != nil {
			re.SetError(err, cmdsutil.ErrNormal)
			return
		}

		err = file.Close()
		if err != nil {
			re.SetError(err, cmdsutil.ErrNormal)
			return
		}

		var pref cid.Prefix
		pref.Version = 1

		format, _, _ := req.Option("format").String()
		switch format {
		case "cbor":
			pref.Codec = cid.DagCBOR
		case "protobuf":
			pref.Codec = cid.DagProtobuf
		case "raw":
			pref.Codec = cid.Raw
		case "v0":
			pref.Version = 0
			pref.Codec = cid.DagProtobuf
		default:
			re.SetError(fmt.Errorf("unrecognized format: %s", format), cmdsutil.ErrNormal)
			return
		}

		mhtype, _, _ := req.Option("mhtype").String()
		mhtval, ok := mh.Names[mhtype]
		if !ok {
			re.SetError(fmt.Errorf("unrecognized multihash function: %s", mhtype), cmdsutil.ErrNormal)
			return
		}
		pref.MhType = mhtval

		mhlen, _, err := req.Option("mhlen").Int()
		if err != nil {
			re.SetError(err, cmdsutil.ErrNormal)
			return
		}
		pref.MhLength = mhlen

		bcid, err := pref.Sum(data)
		if err != nil {
			re.SetError(err, cmdsutil.ErrNormal)
			return
		}

		b, err := blocks.NewBlockWithCid(data, bcid)
		if err != nil {
			re.SetError(err, cmdsutil.ErrNormal)
			return
		}
		log.Debugf("BlockPut key: '%q'", b.Cid())

		k, err := n.Blocks.AddBlock(b)
		if err != nil {
			re.SetError(err, cmdsutil.ErrNormal)
			return
		}

		re.Emit(&BlockStat{
			Key:  k.String(),
			Size: len(data),
		})
	},
	Encoders: map[cmds.EncodingType]func(cmds.Response) func(io.Writer) cmds.Encoder{
		cmds.Text: cmds.MakeEncoder(func(w io.Writer, v interface{}) error {
			bs := v.(*BlockStat)
			_, err := fmt.Fprintf(w, "%s\n", bs.Key)
			return err
		}),
	},
	Type: BlockStat{},
}

func getBlockForKey(req cmds.Request, skey string) (blocks.Block, error) {
	if len(skey) == 0 {
		return nil, fmt.Errorf("zero length cid invalid")
	}

	n, err := req.InvocContext().GetNode()
	if err != nil {
		return nil, err
	}

	c, err := cid.Decode(skey)
	if err != nil {
		return nil, err
	}

	b, err := n.Blocks.GetBlock(req.Context(), c)
	if err != nil {
		return nil, err
	}

	log.Debugf("ipfs block: got block with key: %s", b.Cid())
	return b, nil
}

var blockRmCmd = &cmds.Command{
	Helptext: cmdsutil.HelpText{
		Tagline: "Remove IPFS block(s).",
		ShortDescription: `
'ipfs block rm' is a plumbing command for removing raw ipfs blocks.
It takes a list of base58 encoded multihashs to remove.
`,
	},
	Arguments: []cmdsutil.Argument{
		cmdsutil.StringArg("hash", true, true, "Bash58 encoded multihash of block(s) to remove."),
	},
	Options: []cmdsutil.Option{
		cmdsutil.BoolOption("force", "f", "Ignore nonexistent blocks.").Default(false),
		cmdsutil.BoolOption("quiet", "q", "Write minimal output.").Default(false),
	},
	Run: func(req cmds.Request, re cmds.ResponseEmitter) {
		n, err := req.InvocContext().GetNode()
		if err != nil {
			re.SetError(err, cmdsutil.ErrNormal)
			return
		}
		hashes := req.Arguments()
		force, _, _ := req.Option("force").Bool()
		quiet, _, _ := req.Option("quiet").Bool()
		cids := make([]*cid.Cid, 0, len(hashes))
		for _, hash := range hashes {
			c, err := cid.Decode(hash)
			if err != nil {
				re.SetError(fmt.Errorf("invalid content id: %s (%s)", hash, err), cmdsutil.ErrNormal)
				return
			}

			cids = append(cids, c)
		}
		ch, err := util.RmBlocks(n.Blockstore, n.Pinning, cids, util.RmBlocksOpts{
			Quiet: quiet,
			Force: force,
		})
		if err != nil {
			re.SetError(err, cmdsutil.ErrNormal)
			return
		}
		go func() {
			for v := range ch {
				err := re.Emit(v)
				if err != nil {
					// TODO keks does that even work here? it definitely should!
					re.SetError(err, cmdsutil.ErrNormal)
				}
			}
		}()
	},
	PostRun: map[cmds.EncodingType]func(cmds.Request, cmds.Response) cmds.Response{
		cmds.Text: func(req cmds.Request, res cmds.Response) cmds.Response {
			if res.Error() != nil {
				return res
			}

			re, res_ := cmds.NewChanResponsePair(req)

			outChan := make(chan interface{})

			go func() {
				defer close(outChan)

				for {
					v, err := res.Next()
					if err == io.EOF {
						return
					}
					if err != nil {
						re.SetError(err, cmdsutil.ErrNormal)
						return
					}

					outChan <- v
				}
			}()

			err := util.ProcRmOutput(outChan, os.Stdout, os.Stderr)
			if err != nil {
				re.SetError(err, cmdsutil.ErrNormal)
			}

			return res_
		},
	},
	Type: util.RemovedBlock{},
}
