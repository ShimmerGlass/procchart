package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/shirou/gopsutil/process"
	"github.com/wcharczuk/go-chart"
)

func main() {
	pid := flag.Int("pid", 0, "Process PID")
	interval := flag.Duration("interval", time.Second, "Poll interval")
	duration := flag.Duration("duration", 5*time.Minute, "Duration, 0 for infinite (^C to stop)")
	out := flag.String("out", "graph.png", "Output file")
	flag.Parse()

	p, err := process.NewProcess(int32(*pid))
	if err != nil {
		log.Fatal(err)
	}

	stop := make(chan struct{})
	if *duration > 0 {
		time.AfterFunc(*duration, func() {
			close(stop)
		})
	}
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		close(stop)
	}()

	cpuSeries := []float64{}
	memSeries := []float64{}
	timeSeries := []float64{}

	maxMem := 0.0

	t := time.NewTicker(*interval)

	gt := 0.0
MainLoop:
	for {
		fmt.Println("poll")
		select {
		case <-t.C:
		case <-stop:
			break MainLoop
		}

		ctx, cancel := context.WithTimeout(context.Background(), *interval)

		cpu, err := p.PercentWithContext(ctx, 0)
		if err != nil {
			log.Fatal(err)
		}
		cpuSeries = append(cpuSeries, cpu)

		mem, err := p.MemoryInfoWithContext(ctx)
		if err != nil {
			log.Fatal(err)
		}
		memSeries = append(memSeries, float64(mem.RSS))
		if float64(mem.RSS) > maxMem {
			maxMem = float64(mem.RSS)
		}

		gt += interval.Seconds()
		timeSeries = append(timeSeries, gt)

		cancel()
	}

	fmt.Println(cpuSeries, memSeries, timeSeries)

	graph := chart.Chart{
		XAxis: chart.XAxis{
			Name: "Seconds",
			Style: chart.Style{
				Show: true,
			},
			ValueFormatter: func(v interface{}) string {
				return fmt.Sprintf("%.2fs", v)
			},
		},
		YAxis: chart.YAxis{
			Name: "CPU",
			Style: chart.Style{
				Show: true,
			},
			NameStyle: chart.Style{
				Show: true,
			},
			ValueFormatter: func(v interface{}) string {
				return fmt.Sprintf("%.2f%%", v)
			},
		},
		YAxisSecondary: chart.YAxis{
			Name: "RAM",
			Style: chart.Style{
				Show: true,
			},
			NameStyle: chart.Style{
				Show: true,
			},
			Range: &chart.ContinuousRange{
				Min: 0,
				Max: maxMem,
			},
			ValueFormatter: func(v interface{}) string {
				f := v.(float64)
				return humanize.Bytes(uint64(f))
			},
		},
		Background: chart.Style{
			Padding: chart.Box{
				Top:  20,
				Left: 20,
			},
		},
		Series: []chart.Series{
			chart.ContinuousSeries{
				Name: "CPU",
				Style: chart.Style{
					Show:        true,
					StrokeColor: chart.GetDefaultColor(1),
				},
				YAxis:   chart.YAxisPrimary,
				YValues: cpuSeries,
				XValues: timeSeries,
			},
			chart.ContinuousSeries{
				Name: "RAM",
				Style: chart.Style{
					Show:        true,
					StrokeColor: chart.GetDefaultColor(0),
				},
				YAxis:   chart.YAxisSecondary,
				YValues: memSeries,
				XValues: timeSeries,
			},
		},
	}

	graph.Elements = []chart.Renderable{
		chart.Legend(&graph),
	}

	f, err := os.OpenFile(*out, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	err = graph.Render(chart.PNG, f)
	if err != nil {
		log.Fatal(err)
	}
}
