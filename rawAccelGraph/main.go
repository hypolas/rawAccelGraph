package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"runtime"
	"strconv"

	_ "embed"
	"path/filepath"
	"text/template"

	"fyne.io/fyne/v2/dialog"

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
	RightBottom    fyne.Container
	LabelSlider    map[float64]*canvas.Text
	Sliders        map[float64]*widget.Slider
	SliderAbs      map[float64]*canvas.Text
}

type Settings struct {
	Collumns     *widget.Entry
	Abcisses     *widget.Entry
	OrdonneesMax *widget.Entry
	OrdonneesMin *widget.Entry
	Result       *widget.Entry
}

type RawAccelData struct {
	Data              map[int]string
	DataBindingFloat  map[float64]binding.Float
	DataBindingString map[float64]binding.String
	DataMap           binding.UntypedMap
	Ordon             map[float64]float64
}

type Config struct {
	ConfAbcisses     string
	ConfOrdonneesMax string
	ConfOrdonneesMin string
	ConfCollumns     string
	ConfResult       string
	ConfGraph        map[float64]float64
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
	//Global App et Window setting
	fyneApp.App = app.New()
	fyneApp.Window = fyneApp.App.NewWindow("Raw Accel Data generator by Nicolas HYPOLITE")
	fyneApp.App.Settings().SetTheme(theme.DarkTheme())
	fyneApp.Window.SetIcon(resourceIconPng)

	// Init some vars
	rawAccel.Data = make(map[int]string)
	rawAccel.DataBindingFloat = make(map[float64]binding.Float)
	rawAccel.DataBindingString = make(map[float64]binding.String)
	rawAccel.Ordon = make(map[float64]float64)
	ui.Sliders = make(map[float64]*widget.Slider)
	ui.LabelSlider = make(map[float64]*canvas.Text)
	ui.SliderAbs = make(map[float64]*canvas.Text)

	ui.LeftContainer = container.NewVBox(settings(), &widget.Separator{}, genUIConfig(), result())
	genGraph(false)

	// Load Default Config
	loadConfig("current.yml")

	result := container.NewHSplit(ui.LeftContainer, &ui.RightContainer)
	result.Offset = 0.1

	fyneApp.Window.Resize(fyne.NewSize(1000, 600))

	fyneApp.Window.SetMainMenu(createMenu())
	fyneApp.Window.SetContent(result)

	fyneApp.Window.ShowAndRun()
}

func settings() *fyne.Container {
	set.Collumns = widget.NewEntry()
	set.Collumns.Text = "15"

	set.Abcisses = widget.NewEntry()
	set.Abcisses.Text = "251"

	set.OrdonneesMax = widget.NewEntry()
	set.OrdonneesMax.Text = "2"

	set.OrdonneesMin = widget.NewEntry()
	set.OrdonneesMin.Text = "0.17"

	form := &widget.Form{
		Items: []*widget.FormItem{
			{Text: "Precize", Widget: set.Collumns},
			{Text: "Input Speed", Widget: set.Abcisses},
			{Text: "Ratio Min", Widget: set.OrdonneesMin},
			{Text: "Ratio Max", Widget: set.OrdonneesMax}},
		OnSubmit: func() {
			rawAccel.Data = map[int]string{}
			genGraph(false)
			ui.RightContainer.Refresh()
		},
	}
	largeur := container.NewHScroll(form)
	largeur.SetMinSize(fyne.Size{Width: 170})
	returnCon := container.NewVBox(largeur)
	return returnCon
}

func result() *fyne.Container {
	set.Result = widget.NewMultiLineEntry()
	set.Result.PlaceHolder = "Data to copy in rawAccel"
	scroll := container.NewVScroll(set.Result)
	scroll.SetMinSize(fyne.Size{Height: 300})

	bottomBox := container.NewHBox(
		&widget.Separator{},
		widget.NewButtonWithIcon("   Copy   ", theme.ContentCopyIcon(), func() {
			fyneApp.Window.Clipboard().SetContent(set.Result.Text)
		}),
	)

	aCoffe := container.NewHBox(
		&widget.Separator{},
		widget.NewButtonWithIcon("Give a coffe", theme.VisibilityIcon(), func() {
			openURL("https://www.buymeacoffee.com/laslite")
		}),
	)

	result := container.NewVBox(scroll, container.NewCenter(bottomBox), container.NewCenter(aCoffe))

	return result
}

func genGraph(loadFromSave bool) {
	var collumnsInc float64
	var nbcollumns int

	nbcollumns, _ = strconv.Atoi(set.Collumns.Text)

	sizeAbisses, err := strconv.Atoi(set.Abcisses.Text)
	errorDialog(err)
	collumnsInc = float64(sizeAbisses) / float64(nbcollumns)
	ui.RightContainer = *container.NewGridWithColumns(nbcollumns + 1)

	increment := 1.0
	sizeOrdonnee, err := strconv.Atoi(set.OrdonneesMax.Text)
	errorDialog(err)

	for i := 0; i < nbcollumns+1; i++ {
		currentInc := increment

		rawAccel.DataBindingFloat[currentInc] = binding.NewFloat()
		min, _ := strconv.ParseFloat(set.OrdonneesMin.Text, 8)
		ui.Sliders[currentInc] = widget.NewSlider(min, float64(sizeOrdonnee))
		ui.Sliders[currentInc].Orientation = 1
		ui.Sliders[currentInc].Step = float64(sizeOrdonnee) / 2160
		ui.Sliders[currentInc].Refresh()
		round, err := strconv.Atoi(strconv.FormatFloat(increment, 'f', 0, 64))
		errorDialog(err)

		if loadFromSave && importConf.ConfGraph[currentInc] != min {
			ui.Sliders[currentInc].Value = importConf.ConfGraph[currentInc]
			cordon := strconv.FormatFloat(importConf.ConfGraph[currentInc], 'f', 2, 64)
			rawAccel.Data[round] = cordon
		}

		ui.Sliders[currentInc].OnChanged = func(f float64) {
			ui.LabelSlider[currentInc].Text = strconv.FormatFloat(f, 'f', 3, 64)
			ui.LabelSlider[currentInc].Refresh()
			min, _ := strconv.ParseFloat(set.OrdonneesMin.Text, 8)
			if f == min {
				delete(rawAccel.Data, round)
			} else {
				cordon := strconv.FormatFloat(f, 'f', 3, 64)
				rawAccel.Data[round] = cordon
			}
			genAccelRaw()
		}

		if _, ok := importConf.ConfGraph[currentInc]; ok && loadFromSave && importConf.ConfGraph[currentInc] != 0 {
			ui.LabelSlider[currentInc] = canvas.NewText(strconv.FormatFloat(importConf.ConfGraph[currentInc], 'f', 3, 64), theme.TextColor())
		} else {
			mi := fmt.Sprint(min)
			ui.LabelSlider[currentInc] = canvas.NewText(mi, theme.TextColor())
		}

		ui.LabelSlider[currentInc].TextSize = 12

		ui.SliderAbs[currentInc] = canvas.NewText(strconv.FormatFloat(increment, 'f', 0, 64), theme.TextColor())
		ui.SliderAbs[currentInc].TextSize = 12

		splitCont := container.NewVSplit(container.NewPadded(ui.Sliders[currentInc]),
			container.NewVBox(container.NewCenter(ui.SliderAbs[currentInc]),
				container.NewCenter(ui.LabelSlider[currentInc])))
		splitCont.Offset = 0.99

		ui.RightContainer.Add(splitCont)

		increment = increment + collumnsInc
	}

	ui.RightContainer.Refresh()
}

func genAccelRaw() {
	tpl := &bytes.Buffer{}
	t, err := template.New("").Parse(string(b))
	errorDialog(err)
	err = t.Execute(tpl, rawAccel.Data)
	errorDialog(err)
	set.Result.SetText(tpl.String())
}

func createMenu() *fyne.MainMenu {
	aboutInfo := dialog.NewInformation("About", "Graph data generator for best Mouse Accel software\n \"Raw Accel\"", fyneApp.Window)

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
	exportConf.ConfOrdonneesMax = set.OrdonneesMax.Text
	exportConf.ConfOrdonneesMin = set.OrdonneesMin.Text
	exportConf.ConfCollumns = set.Collumns.Text
	exportConf.ConfResult = set.Result.Text
	exportConf.ConfGraph = make(map[float64]float64)
	for key, value := range ui.Sliders {
		if value.Value != 0 {
			exportConf.ConfGraph[key] = value.Value
		}
	}

	cfg, err := yaml.Marshal(exportConf)
	errorDialog(err)
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

	errorDialog(err)

	return files
}

func loadConfig(conf string) {
	cfg, _ := ioutil.ReadFile("configs/" + conf)
	yaml.Unmarshal(cfg, &importConf)
	set.Abcisses.Text = importConf.ConfAbcisses
	if set.Abcisses.Text == "" {
		set.Abcisses.Text = "250"
	}
	set.Abcisses.Refresh()
	set.OrdonneesMax.Text = importConf.ConfOrdonneesMax
	if set.OrdonneesMax.Text == "" {
		set.OrdonneesMax.Text = "2"
	}
	set.OrdonneesMax.Refresh()
	set.OrdonneesMin.Text = importConf.ConfOrdonneesMin
	if set.OrdonneesMin.Text == "" {
		set.OrdonneesMin.Text = "0"
	}
	set.OrdonneesMin.Refresh()
	set.Collumns.Text = importConf.ConfCollumns
	if set.Collumns.Text == "" {
		set.Collumns.Text = "15"
	}
	set.Collumns.Refresh()
	set.Result.Text = importConf.ConfResult
	set.Result.Refresh()
	ui.RightContainer.Refresh()
	genGraph(true)
}

func errorDialog(err error) {
	if err != nil {
		errorDial := dialog.NewError(err, fyneApp.Window)
		errorDial.Show()
	}
}

func openURL(url string) {
	var err error
	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("unsupported platform")
		errorDialog(err)
	}
	errorDialog(err)
	cmd := exec.Command("open", url)
	cmd.Start()
}
