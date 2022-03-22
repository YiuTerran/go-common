package debugutil

import (
	"fmt"
	"github.com/YiuTerran/go-common/base/log"
	"os"
	"path"
	"runtime/pprof"
	"time"
)

func profileName(prefix, suffix string) string {
	now := time.Now()
	return path.Join(prefix,
		fmt.Sprintf("%d%02d%02d_%02d_%02d_%02d",
			now.Year(),
			now.Month(),
			now.Day(),
			now.Hour(),
			now.Minute(),
			now.Second()), suffix)
}

const (
	CmdStartCpuProfile = "startCpu"
	CmdStopCpuProfile  = "stopCpu"
	CmdDumpGoRoutine   = "goroutine"
	CmdDumpHeap        = "heap"
	CmdDumpThread      = "thread"
	CmdDumpBlock       = "block"
)

var (
	nameSuffix = map[string]string{
		CmdStartCpuProfile: ".cpuprof",
		CmdDumpGoRoutine:   ".gprof",
		CmdDumpHeap:        ".hprof",
		CmdDumpThread:      ".tprof",
		CmdDumpBlock:       ".bprof",
	}
)

// PProfCmd 执行命令
func PProfCmd(cmd string, params ...string) {
	p := "/tmp"
	if cmd != CmdStopCpuProfile && len(params) > 0 {
		p = params[0]
	}
	var (
		fp  *os.File
		err error
		pp  *pprof.Profile
	)
	if cmd != CmdStopCpuProfile {
		if suffix, ok := nameSuffix[cmd]; !ok {
			log.Error("pprof cmd invalid:%s", cmd)
			return
		} else {
			fn := profileName(p, suffix)
			if fp, err = os.Create(fn); err != nil {
				log.Error("fail to create %s, error:%s", fn, err)
				if fp != nil {
					_ = fp.Close()
				}
				return
			}
		}
	}
	defer fp.Close()
	switch cmd {
	case CmdStartCpuProfile:
		err = pprof.StartCPUProfile(fp)
	case CmdStopCpuProfile:
		pprof.StopCPUProfile()
	case CmdDumpGoRoutine:
		pp = pprof.Lookup("goroutine")
	case CmdDumpHeap:
		pp = pprof.Lookup("heap")
	case CmdDumpThread:
		pp = pprof.Lookup("threadcreate")
	case CmdDumpBlock:
		pp = pprof.Lookup("block")
	default:
		log.Error("unknown pprof command:%s", cmd)
		return
	}
	if pp != nil {
		if err = pp.WriteTo(fp, 0); err != nil {
			log.Error("fail to write pprof result, error:%s", err)
		}
	}
}
