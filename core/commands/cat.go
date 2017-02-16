package commands

import (
	"io"
	"os"

	cmds "github.com/ipfs/go-ipfs-cmds"
	"github.com/ipfs/go-ipfs-cmds/cmdsutil"
	core "github.com/ipfs/go-ipfs/core"
	coreunix "github.com/ipfs/go-ipfs/core/coreunix"

	context "context"
)

const progressBarMinSize = 1024 * 1024 * 8 // show progress bar for outputs > 8MiB

var CatCmd = &cmds.Command{
	Helptext: cmdsutil.HelpText{
		Tagline:          "Show IPFS object data.",
		ShortDescription: "Displays the data contained by an IPFS or IPNS object(s) at the given path.",
	},

	Arguments: []cmdsutil.Argument{
		cmdsutil.StringArg("ipfs-path", true, true, "The path to the IPFS object(s) to be outputted.").EnableStdin(),
	},
	Run: func(req cmds.Request, re cmds.ResponseEmitter) {
		log.Debugf("cat: RespEm type is %T", re)
		node, err := req.InvocContext().GetNode()
		if err != nil {
			re.SetError(err, cmdsutil.ErrNormal)
			return
		}

		if !node.OnlineMode() {
			if err := node.SetupOfflineRouting(); err != nil {
				re.SetError(err, cmdsutil.ErrNormal)
				return
			}
		}

		readers, length, err := cat(req.Context(), node, req.Arguments())
		log.Debug("cat returned ", err)

		if err != nil {
			re.SetError(err, cmdsutil.ErrNormal)
			return
		}

		/*
			if err := corerepo.ConditionalGC(req.Context(), node, length); err != nil {
				res.SetError(err, cmdsutil.ErrNormal)
				return
			}
		*/

		re.SetLength(length)

		reader := io.MultiReader(readers...)
		go func() {
			re.Emit(reader)
			re.Close()
		}()
	},
	PostRun: map[cmds.EncodingType]func(cmds.Request, cmds.Response) cmds.Response{
		cmds.CLI: func(req cmds.Request, res cmds.Response) cmds.Response {
			if res.Length() < progressBarMinSize {
				return res
			}

			re, res_ := cmds.NewChanResponsePair(req)

			v, err := res.Next()
			if err != nil {
			}

			r, ok := v.(io.Reader)
			if !ok {
			}

			bar, reader := progressBarForReader(os.Stderr, r, int64(res.Length()))
			bar.Start()

			re.Emit(reader)

			return res_
		},
	},
}

func cat(ctx context.Context, node *core.IpfsNode, paths []string) ([]io.Reader, uint64, error) {
	readers := make([]io.Reader, 0, len(paths))
	length := uint64(0)
	for _, fpath := range paths {
		read, err := coreunix.Cat(ctx, node, fpath)
		if err != nil {
			return nil, 0, err
		}
		readers = append(readers, read)
		length += uint64(read.Size())
	}
	return readers, length, nil
}
