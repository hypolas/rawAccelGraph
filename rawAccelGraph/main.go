package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"

	_ "embed"
	"fyne.io/fyne/v2/dialog"
	"path/filepath"
	"text/template"

	"gopkg.in/yaml.v3"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type Graph struct {
	LeftContainer  *fyne.Container
	RightContainer fyne.Container
	LabelSlider    map[float64]*canvas.Text
	Sliders        map[float64]*widget.Slider
	SliderAbs      map[float64]*canvas.Text
}

type Settings struct {
	Collumns  *widget.Entry
	Abcisses  *widget.Entry
	Ordonnees *widget.Entry
	Result    *widget.Entry
}

type RawAccelData struct {
	Data              map[int]string
	DataBindingFloat  map[float64]binding.Float
	DataBindingString map[float64]binding.String
	DataMap           binding.UntypedMap
	Ordon             map[float64]float64
}

type Config struct {
	ConfAbcisses string
	ConfOrdonnee string
	ConfCollumns string
	ConfResult   string
	ConfGraph    map[float64]float64
}

type FyneApp struct {
	App    fyne.App
	Window fyne.Window
}

var ui Graph
var set Settings
var rawAccel RawAccelData
var importConf Config
var fyneApp FyneApp

//go:embed rawAccell.tmpl
var b []byte

var tpl bytes.Buffer

func init() {
	os.Setenv("FYNE_THEME", "dark")
}

func main() {
	fyneApp.App = app.New()
	fyneApp.Window = fyneApp.App.NewWindow("Raw Accel Data generator by Nicolas HYPOLITE")
	fyneApp.App.Settings().SetTheme(theme.DarkTheme())
	rawAccel.Data = make(map[int]string)
	rawAccel.DataBindingFloat = make(map[float64]binding.Float)
	rawAccel.DataBindingString = make(map[float64]binding.String)
	rawAccel.Ordon = make(map[float64]float64)
	ui.Sliders = make(map[float64]*widget.Slider)
	ui.LabelSlider = make(map[float64]*canvas.Text)
	ui.SliderAbs = make(map[float64]*canvas.Text)

	ui.LeftContainer = container.NewVBox(settings(), &widget.Separator{}, genUIConfig(), result())
	genGraph(false)

	result := container.NewHSplit(ui.LeftContainer, &ui.RightContainer)
	result.Offset = 0.1

	fyneApp.Window.Resize(fyne.NewSize(1000, 800))

	fyneApp.Window.SetMainMenu(createMenu())
	fyneApp.Window.SetContent(result)

	fyneApp.Window.ShowAndRun()
}

func settings() *widget.Form {
	set.Collumns = widget.NewEntry()
	set.Collumns.Text = "0"

	set.Abcisses = widget.NewEntry()
	set.Abcisses.Text = "251"

	set.Ordonnees = widget.NewEntry()
	set.Ordonnees.Text = "2"

	form := &widget.Form{
		Items: []*widget.FormItem{
			{Text: "Precize", Widget: set.Collumns},
			{Text: "Input Speed", Widget: set.Abcisses},
			{Text: "Ratio of Output", Widget: set.Ordonnees}},
		OnSubmit: func() {
			rawAccel.Data = map[int]string{}
			genGraph(false)
			ui.RightContainer.Refresh()
		},
	}
	return form
}

func result() *container.Scroll {
	set.Result = widget.NewMultiLineEntry()
	set.Result.PlaceHolder = "Data to copy in rawAccel"
	scroll := container.NewVScroll(set.Result)
	scroll.SetMinSize(fyne.Size{Height: 300})
	return scroll
}

func genGraph(loadFromSave bool) {
	var collumnsInc float64
	nbcollumns, _ := strconv.Atoi(set.Collumns.Text)
	if set.Collumns.Text == "0" {
		nbcollumns = 20
		set.Collumns.SetText(strconv.Itoa(nbcollumns))
	}

	sizeAbisses, _ := strconv.Atoi(set.Abcisses.Text)
	collumnsInc = float64(sizeAbisses) / float64(nbcollumns)
	ui.RightContainer = *container.NewGridWithColumns(nbcollumns + 1)

	increment := 1.0
	sizeOrdonnee, _ := strconv.Atoi(set.Ordonnees.Text)

	for i := 0; i < nbcollumns+1; i++ {
		currentInc := increment

		rawAccel.DataBindingFloat[currentInc] = binding.NewFloat()

		ui.Sliders[currentInc] = widget.NewSlider(0, float64(sizeOrdonnee))
		ui.Sliders[currentInc].Orientation = 1
		ui.Sliders[currentInc].Step = float64(sizeOrdonnee) / 2160
		round, _ := strconv.Atoi(strconv.FormatFloat(increment, 'f', 0, 64))

		if loadFromSave && importConf.ConfGraph[currentInc] != 0 {
			ui.Sliders[currentInc].Value = importConf.ConfGraph[currentInc]
			cordon := strconv.FormatFloat(importConf.ConfGraph[currentInc], 'f', 2, 64)
			rawAccel.Data[round] = cordon
		}

		ui.Sliders[currentInc].OnChanged = func(f float64) {
			ui.LabelSlider[currentInc].Text = strconv.FormatFloat(f, 'f', 2, 64)
			ui.LabelSlider[currentInc].Refresh()
			if f == 0 {
				delete(rawAccel.Data, round)
			} else {
				cordon := strconv.FormatFloat(f, 'f', 2, 64)
				rawAccel.Data[round] = cordon
			}
			genAccelRaw()
		}

		if loadFromSave && importConf.ConfGraph[currentInc] != 0 {
			ui.LabelSlider[currentInc].Text = strconv.FormatFloat(importConf.ConfGraph[currentInc], 'f', 2, 64)
		} else {
			ui.LabelSlider[currentInc] = canvas.NewText("0.00", theme.TextColor())
		}

		ui.LabelSlider[currentInc].TextSize = 12

		ui.SliderAbs[currentInc] = canvas.NewText(strconv.FormatFloat(increment, 'f', 0, 64), theme.TextColor())
		ui.SliderAbs[currentInc].TextSize = 12

		ui.RightContainer.Add(container.NewMax(ui.Sliders[currentInc],
			container.NewVBox(ui.SliderAbs[currentInc],
				ui.LabelSlider[currentInc])))
		increment = increment + collumnsInc
	}

	ui.RightContainer.Refresh()
}

func genAccelRaw() {
	tpl := &bytes.Buffer{}
	t, _ := template.New("").Parse(string(b))
	_ = t.Execute(tpl, rawAccel.Data)
	set.Result.SetText(tpl.String())
}

func createMenu() *fyne.MainMenu {
	aboutInfo := dialog.NewInformation("About", "Graph data generator for best mouse accell software\n raw Accel", fyneApp.Window)

	about := &fyne.MenuItem{
		Label: "About",
		Action: func() {
			aboutInfo.Show()
		},
	}

	menuItem := &fyne.Menu{
		Label: "File",
		Items: nil, // we will add sub items in next video
	}

	menuItem.Items = append(menuItem.Items, about)

	menu := fyne.NewMainMenu(menuItem)
	return menu
}

func genUIConfig() *fyne.Container {
	loadBtn := widget.NewSelect(listConfigs(), func(s string) {
		loadConfig(s)
	})
	saveBtn := widget.NewButton("Save", func() {
		saveConfig()
	})
	saveAsBtn := widget.NewButton("Save as", func() {
		saveConfig()
	})
	return container.NewVBox(container.NewMax(loadBtn), container.NewHSplit(saveBtn, saveAsBtn))
}

func saveConfig() {
	_ = os.Mkdir("configs/", 0755)
	var exportConf Config
	exportConf.ConfAbcisses = set.Abcisses.Text
	exportConf.ConfOrdonnee = set.Ordonnees.Text
	exportConf.ConfCollumns = set.Collumns.Text
	exportConf.ConfResult = set.Result.Text
	exportConf.ConfGraph = make(map[float64]float64)
	for key, value := range ui.Sliders {
		if value.Value != 0 {
			exportConf.ConfGraph[key] = value.Value
		}
	}

	cfg, _ := yaml.Marshal(exportConf)
	ioutil.WriteFile("configs/current.yml", cfg, 0755)
}

func listConfigs() []string {
	_ = os.Mkdir("configs/", 0755)
	var files []string

	root := "configs/"

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			files = append(files, info.Name())
		}
		return nil
	})

	if err != nil {
		panic(err)
	}
	return files
}

func loadConfig(conf string) {
	cfg, _ := ioutil.ReadFile("configs/" + conf)
	yaml.Unmarshal(cfg, &importConf)
	set.Abcisses.Text = importConf.ConfAbcisses
	fmt.Println(set.Abcisses.Text)
	set.Ordonnees.Text = importConf.ConfOrdonnee
	fmt.Println(set.Ordonnees.Text)
	set.Collumns.Text = importConf.ConfCollumns
	set.Result.Text = importConf.ConfResult
	set.Result.Refresh()
	genGraph(true)
	ui.LeftContainer.Refresh()
}
