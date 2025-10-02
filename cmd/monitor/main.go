// Copyright ©2025 Dan Kortschak. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The monitor command is a demonstration of heart and pmd
// packages for heart rate and ECG data.
package main

import (
	"context"
	"flag"
	"fmt"
	"image"
	"log"
	"os"
	"os/signal"
	"slices"

	"gioui.org/app"
	"gioui.org/font/gofont"
	"gioui.org/io/event"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/explorer"
	"tinygo.org/x/bluetooth"
)

func main() {
	adapter := bluetooth.DefaultAdapter
	err := adapter.Enable()
	if err != nil {
		fmt.Printf("failed to enable bluetooth: %v", err)
		os.Exit(1)
	}

	addr := flag.String("addr", "", "sensor bluetooth address")
	flag.Parse()
	if *addr == "" {
		flag.Usage()
		os.Exit(2)
	}
	var macAddr bluetooth.Address
	err = macAddr.UnmarshalText([]byte(*addr))
	if err != nil {
		flag.Usage()
		os.Exit(2)
	}

	fmt.Println("scanning...")
	var dev bluetooth.Device
	err = adapter.Scan(func(adapter *bluetooth.Adapter, found bluetooth.ScanResult) {
		if !slices.ContainsFunc(found.ManufacturerData(), func(m bluetooth.ManufacturerDataElement) bool {
			const polarElectroOY = 0x6b // https://bitbucket.org/bluetooth-SIG/public/src/05be78f4ef6461cce0370663adf778613a1754eb/assigned_numbers/company_identifiers/company_identifiers.yaml#lines-11148:11149
			return m.CompanyID == polarElectroOY
		}) {
			return
		}
		if found.Address == macAddr {
			fmt.Printf(`
found device:
  mac: %s rss: %d
  name: %q
  manufacturer data: %v
  service: %#v
  payload: %#v
`,
				found.Address, found.RSSI,
				found.LocalName(),
				manData(found.ManufacturerData()),
				found.ServiceData(),
				found.AdvertisementPayload.Bytes(),
			)
			dev, err = adapter.Connect(found.Address, bluetooth.ConnectionParams{})
			if err != nil {
				fmt.Printf("failed to connect: %v", err)
				return
			}
			adapter.StopScan()
		}
	})
	defer dev.Disconnect()

	update := make(chan image.Image)
	m, err := newMonitor(context.Background(), dev, update)
	if err != nil {
		log.Fatal(err)
	}
	defer m.Close()

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	go func() {
		<-interrupt
		m.Close()
		os.Exit(0)
	}()

	go func() {
		w := new(app.Window)
		w.Option(app.Title("ECG"), app.Size(296, 128))
		if err := loop(w, update); err != nil {
			log.Fatal(err)
		}
		m.Close()
		os.Exit(0)
	}()
	app.Main()
}

func manData(m []bluetooth.ManufacturerDataElement) []string {
	s := make([]string, len(m))
	for i, d := range m {
		s[i] = fmt.Sprintf("%#x", d.Data)
	}
	return s
}

func loop(w *app.Window, update chan image.Image) error {
	expl := explorer.NewExplorer(w)
	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))

	events := make(chan event.Event)
	ack := make(chan struct{})

	go func() {
		for {
			ev := w.Event()
			events <- ev
			<-ack
			if _, ok := ev.(app.DestroyEvent); ok {
				return
			}
		}
	}()
	var img image.Image
	var ops op.Ops
	for {
		select {
		case img = <-update:
			w.Invalidate()
		case e := <-events:
			expl.ListenEvents(e)
			switch e := e.(type) {
			case app.DestroyEvent:
				ack <- struct{}{}
				return e.Err
			case app.FrameEvent:
				gtx := app.NewContext(&ops, e)
				layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						if img == nil {
							return layout.Dimensions{}
						}
						return widget.Image{
							Src: paint.NewImageOp(img),
							Fit: widget.Contain,
						}.Layout(gtx)
					}),
				)
				e.Frame(gtx.Ops)
			}
			ack <- struct{}{}
		}
	}
}
