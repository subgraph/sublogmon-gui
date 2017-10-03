//
// Might require this function to be implemented in gotk3:
// gtk_tree_sortable_set_sort_func()
//

package main

import (
	"github.com/gotk3/gotk3/gtk"
	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/pango"
	"log"
	"fmt"
	"strings"
	"sort"
	"os"
	"io/ioutil"
	"encoding/json"
	"regexp"
	"time"
	"os/user"
	"strconv"
)


type sublogTabView struct {
	LogLevel string
	LS *gtk.ListStore
}

type logSuppression struct {
	Description string
	Wildcard string
	Metadata map[string]string
	Count int
}

type logBuffered struct {
	Line string
	OrigLine string
	Metadata map[string]string
	Timestamps []time.Time
	LineIdx int
}

type slPreferences struct {
	Winheight uint
	Winwidth uint
	Wintop uint
	Winleft uint
	Logfile string
	CollapseWin uint
}


var allTabs map[string]sublogTabView
var userPrefs slPreferences
var allSuppressions []logSuppression
var logBuffer = map[string][]logBuffered { "critical": {}, "alert": {}, "default": {}, "all": {} }
var mainWin *gtk.Window
var Notebook *gtk.Notebook
var globalLS *gtk.ListStore
var outLogFile *os.File = nil
var colScale *gtk.Scale = nil

var allLogLevels = []string { "critical", "alert", "default", "all" }

var colorLevelMap = map[string]string{
	"critical": "red",
	"alert": "orange",
	"default": "black",
}

func openOutLog(filename string) bool {
	var err error
	outLogFile, err = os.OpenFile(filename, os.O_APPEND|os.O_WRONLY|os.O_CREATE,0600)

	if err != nil {
		promptError("Could not open log file for writing: "+err.Error())
		outLogFile = nil
		return false
	}

	return true
}

func writeOutLog(data string) {

	if outLogFile == nil {
		return
	}

	if _, err := outLogFile.WriteString(data); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing to output log file:", err)
	}

}

func promptInfo(msg string) {
	dialog := gtk.MessageDialogNew(mainWin, 0, gtk.MESSAGE_INFO, gtk.BUTTONS_OK, "Displaying full log info:")
//	dialog.SetDefaultGeometry(500, 200)

	tv, err := gtk.TextViewNew()

	if err != nil {
		log.Fatal("Unable to create TextView:", err)
	}

	tvbuf, err := tv.GetBuffer()

	if err != nil {
		log.Fatal("Unable to get buffer:", err)
	}

	tvbuf.SetText(msg)
	tv.SetEditable(false)
	tv.SetWrapMode(gtk.WRAP_WORD)

	scrollbox, err := gtk.ScrolledWindowNew(nil, nil)

	if err != nil {
		log.Fatal("Unable to create scrolled window:", err)
	}

	scrollbox.Add(tv)
	scrollbox.SetSizeRequest(600, 100)

	box, err := dialog.GetContentArea()

	if err != nil {
		log.Fatal("Unable to get content area of dialog:", err)
	}

	box.Add(scrollbox)
	dialog.ShowAll()
	dialog.Run()
	dialog.Destroy()
//self.set_default_size(150, 100)
}

func promptChoice(msg string) int {
	dialog := gtk.MessageDialogNew(mainWin, 0, gtk.MESSAGE_ERROR, gtk.BUTTONS_YES_NO, msg)
	result := dialog.Run()
	dialog.Destroy()
	return result
}

func promptError(msg string) {
	dialog := gtk.MessageDialogNew(mainWin, 0, gtk.MESSAGE_ERROR, gtk.BUTTONS_CLOSE, "Error: %s", msg)
	dialog.Run()
	dialog.Destroy()
}

func getConfigPath() string {
	usr, err := user.Current()

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: could not determine location of user preferences file:", err, "\n");
		return ""
	}

	prefPath := usr.HomeDir + "/.sublogmon.json"
	return prefPath
}

func savePreferences() bool {
	usr, err := user.Current()

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: could not determine location of user preferences file:", err, "\n");
		return false
	}

	prefPath := usr.HomeDir + "/.sublogmon.json"

	jsonPrefs, err := json.Marshal(userPrefs)

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: could not generate user preferences data:", err, "\n")
		return false
	}

	err = ioutil.WriteFile(prefPath, jsonPrefs, 0644)

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: could not save user preferences data:", err, "\n")
		return false
	}

	return true
}


func loadPreferences() bool {
	usr, err := user.Current()

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: could not determine location of user preferences file: %v", err, "\n");
		return false
	}

	prefPath := usr.HomeDir + "/.sublogmon.json"
	fmt.Println("xxxxxxxxxxxxxxxxxxxxxx preferences path = ", prefPath)

	jfile, err := ioutil.ReadFile(prefPath)

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: could not read preference data from file: %v", err, "\n")
		return false
	}

	err = json.Unmarshal(jfile, &userPrefs)

	if err != nil {
                fmt.Fprintf(os.Stderr, "Error: could not load preferences data from file: %v", err, "\n")
		return false
	}

	fmt.Println(userPrefs)
	return true
}

func loadSuppressions(filepath string) bool {

	jfile, err := ioutil.ReadFile(filepath)

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: could not read suppressions from file:", err, "\n")
		return false
	}

	err = json.Unmarshal(jfile, &allSuppressions)

	if err != nil {
                fmt.Fprintf(os.Stderr, "Error: could not load suppression data from file:", err, "\n")
		return false
	}

	fmt.Fprintf(os.Stderr, "Read a total of %d suppressions from config\n", len(allSuppressions))

	return true
}

func buffer_line(loglevel, line, oline string, metadata map[string]string) (int, logBuffered) {
	found := -1
	now := time.Now()

	fmt.Printf("looking in section %s / len = %d\n", loglevel, len(logBuffer[loglevel]))

	for i := 0; i < len(logBuffer[loglevel]); i++ {

		if logBuffer[loglevel][i].Line == line {
			found = i
			break
		}

	}

	anewbuf := logBuffered { line, oline, metadata, []time.Time{ now }, len(logBuffer["all"]) }
	logBuffer["all"] = append(logBuffer[loglevel], anewbuf)

	if found >= 0 {
		logBuffer[loglevel][found].Timestamps = append(logBuffer[loglevel][found].Timestamps, now)
		return len(logBuffer[loglevel][found].Timestamps), logBuffer[loglevel][found]
	}

	lineno := 0
	lineno = len(logBuffer[loglevel])
//	lineno := allTabs[loglevel].TVBuffer.GetLineCount() - 1

	fmt.Println("_____________ lineno = ", lineno)
	newbuf := logBuffered { line, oline, metadata, []time.Time{ now }, lineno }
	logBuffer[loglevel] = append(logBuffer[loglevel], newbuf)
	return 0, newbuf
}

func appendLogLine(line, oline, loglevel, provider, process string, timestamp int64, section, all bool) {
	thisTab := allTabs[loglevel]

	if thisTab.LS == nil {
		fmt.Println("Got logging data but application was not initialized yet...")
		return
	}

	fmt.Println("heh and then logged something.")
	tss := time.Unix(timestamp / 1000000000, timestamp % 1000000000).Format("02/01/06 15:04:05") + "." + fmt.Sprintf("%d", (timestamp % 1000000000)/10000)

	if section {
		addLogRow(thisTab.LS, 0, tss, loglevel, provider, process, line, oline)
	}

	if all {
		addLogRow(allTabs["all"].LS, 0, tss, loglevel, provider, process, line, oline)
	}

	writeOutLog(tss + "[" + loglevel + "] " + line+"\n")
}

func guiLog(data slmData) {
	fmt.Printf("XXX: loglevel = %s, eventid = %s\n", data.LogLevel, data.EventID)
	suppressed := false
	possibleSuppression := false


	for i := 0; i < len(allSuppressions); i++ {

		if len(allSuppressions[i].Wildcard) == 0 {
//			fmt.Println("XXX: possible regex match: ", allSuppressions[i].Description)
			possibleSuppression = true
		} else {
			matched, err := regexp.MatchString(allSuppressions[i].Wildcard, data.LogLine)

			if err == nil && matched {
				possibleSuppression = true
			}

		}

		if possibleSuppression {

			for mname := range allSuppressions[i].Metadata {

				if allSuppressions[i].Metadata[mname] == "" {
					continue
				}

				matched, err := regexp.MatchString(allSuppressions[i].Metadata[mname], data.Metadata[mname])

				if (err == nil && matched) || (data.Metadata[mname] == allSuppressions[i].Metadata[mname]) {
//					fmt.Println("//////////////////// actually seemed to match = ", data.Metadata[mname])
				} else {
					possibleSuppression = false
				}

			}

			if possibleSuppression {
				fmt.Println("////////// made it all the way through: ", data.LogLine)
				allSuppressions[i].Count++
				update_suppression_count(i, allSuppressions[i].Count)
				suppressed = true
			}

		}

//		fmt.Println("suppressions count for ", allSuppressions[i].Description, " = ", allSuppressions[i].Count)
	}

	if suppressed {
		fmt.Println("*** WAS SUPPRESSED: ", data.LogLine)
		return
	}

	process := ""
	mprocess, exists := data.Metadata["process"]

	if exists {
		process = mprocess
	}

	nbuf, bufentry := buffer_line(data.LogLevel, data.LogLine, data.OrigLogLine, data.Metadata)
//	fmt.Println("ORIG: ", data.OrigLogLine)
//	fmt.Println("---------- nbuf = ", nbuf)

	if nbuf > 0 {
		fmt.Println("+++++++++++++++ should overwrite line: ", bufentry.LineIdx)
		updateRow(allTabs[data.LogLevel].LS, 0, nbuf)
		appendLogLine(data.LogLine, data.OrigLogLine, data.LogLevel, data.EventID, process, data.Timestamp, false, true)
	} else {
		appendLogLine(data.LogLine, data.OrigLogLine, data.LogLevel, data.EventID, process, data.Timestamp, true, true)

		if data.LogLevel == "critical" || data.LogLevel == "alert" {
			dn.show("sysevent", data.LogLine, true)
		}

	}

}

func get_hbox() *gtk.Box {
        hbox, err := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 0)

        if err != nil {
                log.Fatal("Unable to create horizontal box:", err)
        }

        return hbox
}

func get_entry(text string) *gtk.Entry {
        entry, err := gtk.EntryNew()

        if err != nil {
                log.Fatal("Unable to create text entry:", err)
        }

        entry.SetText(text)
        return entry
}

func get_label(text string) *gtk.Label {
	label, err := gtk.LabelNew(text)

	if err != nil {
		log.Fatal("Unable to create label in GUI:", err)
		return nil
	}

	return label
}

func add_all_unique_meta_fields(mmap []string, data map[string]string) []string {

	for i := range data {
		var j = 0

		for j = 0; j < len(mmap); j++ {

			if strings.ToLower(mmap[j]) == strings.ToLower(i) {
				break
			}

		}

		if j == len(mmap) {
			mmap = append(mmap, i)
		}

	}

	return mmap
}


func createColumn(title string, id int) *gtk.TreeViewColumn {
	cellRenderer, err := gtk.CellRendererTextNew()

	if err != nil {
		log.Fatal("Unable to create text cell renderer:", err)
	}

	column, err := gtk.TreeViewColumnNewWithAttribute(title, cellRenderer, "text", id)

	if err != nil {
		log.Fatal("Unable to create cell column:", err)
	}

	column.SetSortColumnID(id)
	column.SetResizable(true)
	return column
}

func createLogListStore(general bool) *gtk.ListStore {
	colData := []glib.Type{glib.TYPE_INT, glib.TYPE_STRING, glib.TYPE_STRING, glib.TYPE_STRING, glib.TYPE_STRING, glib.TYPE_STRING}

	if general {
		colData = []glib.Type{glib.TYPE_INT, glib.TYPE_STRING, glib.TYPE_STRING, glib.TYPE_STRING, glib.TYPE_STRING, glib.TYPE_STRING, glib.TYPE_STRING}
	}

	listStore, err := gtk.ListStoreNew(colData...)

	if err != nil {
		log.Fatal("Unable to create list store:", err)
	}

	return listStore
}

func createListStore(nadded int) *gtk.ListStore {
	colData := []glib.Type{glib.TYPE_INT, glib.TYPE_STRING, glib.TYPE_STRING}

	for n := 0; n < nadded; n++ {
		colData = append(colData, glib.TYPE_STRING)
	}

	listStore, err := gtk.ListStoreNew(colData...)

	if err != nil {
		log.Fatal("Unable to create list store:", err)
	}

	return listStore
}

func addLogRow(listStore *gtk.ListStore, count int, date, level, provider, process, line, oline string) {
	iter := listStore.Append()

	colVals := make([]interface{}, 7)
	colVals[0] = count
	colVals[1] = date
	colVals[2] = level
	colVals[3] = provider
	colVals[4] = process
	colVals[5] = line
	colVals[6] = oline

	colNums := make([]int, len(colVals))

	for n := 0; n < len(colVals); n++ {
		colNums[n] = n
	}

	err := listStore.Set(iter, colNums, colVals)

	if err != nil {
		log.Fatal("Unable to add row:", err)
	}

}

func updateRow(listStore *gtk.ListStore, colno int, data interface{}) {
//	fmt.Println("UPDATE ROW")

	path, err := gtk.TreePathNewFromString(fmt.Sprintf("%d", colno))

	if err != nil {
		log.Fatal("Error looking up row in tree data:", err)
	}

	iter, err := listStore.GetIter(path)

	if err != nil {
		log.Fatal("Error looking up row in tree data by path:", err)
	}

	listStore.SetValue(iter, colno, data)
	return
}

func addSuppRow(listStore *gtk.ListStore, description, wildcard string, metaNames []string, metadata map[string]string) {
	iter := listStore.Append()

	colVals := make([]interface{}, len(metaNames)+3)
	colVals[0] = 0
	colVals[1] = description
	colVals[2] = wildcard

	for n := 0; n < len(metaNames); n++ {
		mval, _ := metadata[metaNames[n]]
		colVals[n+3] = mval
	}

	colNums := make([]int, len(metaNames)+3)

	for n := 0; n < len(metaNames)+3; n++ {
		colNums[n] = n
	}

	err := listStore.Set(iter, colNums, colVals)

	if err != nil {
		log.Fatal("Unable to add row:", err)
	}

}

func update_suppression_count(rownum, val int) {
	listStore := globalLS
	fmt.Println("--------------------- attempting to suppress row = ", rownum)

	ix, _ := listStore.GetIterFirst()

	if listStore.IterNthChild(ix, nil, rownum) {
		listStore.SetValue(ix, 0, val)
	} else {
		fmt.Println("Error: tried to update suppression count for non-existent row: ", rownum)
	}

}


type sortStrings []string

func (s sortStrings) Len() int {
    return len(s)
}

func (s sortStrings) Less(i, j int) bool {
    return s[i] < s[j]
}

func (s sortStrings) Swap(i, j int) {
    s[i], s[j] = s[j], s[i]
}

func setup_settings() {
	box, err := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 0)

	if err != nil {
		log.Fatal("Unable to create settings box:", err)
	}

	scrollbox, err := gtk.ScrolledWindowNew(nil, nil)

	if err != nil {
		log.Fatal("Unable to create settings scrolled window:", err)
	}

	hLabel, err := gtk.LabelNew("Settings")

	if err != nil {
		log.Fatal("Unable to create notebook label:", err)
	}

	scrollbox.Add(box)
	scrollbox.SetSizeRequest(600, 800)

	supPath := "suppressions.json"
	success := loadSuppressions(supPath)

	var allMetadata []string

	tv, err := gtk.TreeViewNew()

	if err != nil {
		log.Fatal("Unable to create treeview:", err)
	}

//	scrollbox.Add(tv)

	h := get_hbox()
	l := get_label("Log to file:")
	e := get_entry(userPrefs.Logfile)
	b, err := gtk.ButtonNewWithLabel("Save")

	if err != nil {
		log.Fatal("Unable to create button:", err)
	}

	h.PackStart(l, false, true, 10)
	h.PackStart(e, false, true, 10)
	h.PackStart(b, false, true, 10)
	h.SetMarginTop(10)
	box.Add(h)

	h = get_hbox()
	colScale, err = gtk.ScaleNewWithRange(gtk.ORIENTATION_HORIZONTAL, 0, 120, 2)

	if err != nil {
		log.Fatal("Unable to create scaler:", err)
	}

	l = get_label("Collapse duplicate log items in interval:")
	h.PackStart(l, false, true, 10)
	h.PackStart(colScale, true, true, 10)
	l = get_label("minutes")
	h.PackStart(l, false, true, 10)
	h.SetMarginTop(0)
	h.SetMarginBottom(20)
	box.Add(h)

	box.Add(tv)

	b.Connect("clicked", func() {
		fmt.Println("CLICKED")

		userPrefs.Logfile, err = e.GetText()

		if err != nil {
			promptError("Unexpected error saving log file info: "+err.Error())
			return
		}

		if openOutLog(userPrefs.Logfile) {
			savePreferences()
		}

	})

	if success {
/*		lString := fmt.Sprintf("Loaded a total of %d suppressions from file: %s\n", len(allSuppressions), supPath)
		sLabel, err := gtk.LabelNew(lString)

		if err != nil {
			log.Fatal("Unable to create notebook label:", err)
		} */
		tv.AppendColumn(createColumn("#", 0))
		tv.AppendColumn(createColumn("Description", 1))
		tv.AppendColumn(createColumn("Wildcard", 2))

		for n := 0; n < len(allSuppressions); n++ {
			allMetadata = add_all_unique_meta_fields(allMetadata, allSuppressions[n].Metadata)
			sort.Sort(sortStrings(allMetadata))
		}

		listStore := createListStore(3+len(allMetadata))
		globalLS = listStore
		tv.SetModel(listStore)

		fmt.Printf("XXX: got %d unique metadata fields\n", len(allMetadata))
		for x := 0; x < len(allMetadata); x++ {
			fmt.Printf("XXX: %s\n", allMetadata[x])
			tv.AppendColumn(createColumn(allMetadata[x], x+3))
		}

		for n := 0; n < len(allSuppressions); n++ {
			addSuppRow(listStore, allSuppressions[n].Description, allSuppressions[n].Wildcard, allMetadata, allSuppressions[n].Metadata)
		}

	}

//	Notebook.AppendPage(box, hLabel)
	Notebook.AppendPage(scrollbox, hLabel)
}

func get_bold_texttag() *gtk.TextTag {
	boldTT, err := gtk.TextTagNew("bold")

	if err != nil {
		log.Fatal("Unable to create text tag for boldface:", err)
	}

	boldTT.SetProperty("weight", pango.WEIGHT_ULTRABOLD)
	return boldTT
}

func get_underline_texttag() *gtk.TextTag {
	ulTT, err := gtk.TextTagNew("underline")

	if err != nil {
		log.Fatal("Unable to create text tag for underline:", err)
	}

	ulTT.SetProperty("underline", pango.UNDERLINE_SINGLE)
	ulTT.SetProperty("size", pango.SCALE_XX_LARGE)
//	ulTT.SetProperty("font_desc", "Sans Italic 20")
	return ulTT
}

func gui_main() {
	if len(os.Args) == 3 && os.Args[1] == "-display" {
		os.Setenv("DISPLAY", os.Args[2])
	}

	loadPreferences()
	gtk.Init(nil)

	// Create a new toplevel window, set its title, and connect it to the "destroy" signal to exit the GTK main loop when it is destroyed.
	mainWin, err := gtk.WindowNew(gtk.WINDOW_TOPLEVEL)

	if err != nil {
		log.Fatal("Unable to create window:", err)
	}

	mainWin.SetTitle("Subgraph Log Monitor")

	mainWin.Connect("destroy", func() {
		fmt.Println("Shutting down...")
		userPrefs.CollapseWin = uint(colScale.GetValue())
		savePreferences()
	        gtk.MainQuit()
	})

	mainWin.Connect("configure-event", func() {
		w, h := mainWin.GetSize()
		userPrefs.Winwidth, userPrefs.Winheight = uint(w), uint(h)
		l, t := mainWin.GetPosition()
		userPrefs.Winleft, userPrefs.Wintop = uint(l), uint(t)
	})

	mainWin.SetPosition(gtk.WIN_POS_CENTER)

	Notebook, err = gtk.NotebookNew()

	if err != nil {
		log.Fatal("Unable to create new notebook:", err)
	}

	allTabs = make(map[string]sublogTabView)

	for _, loglevel := range allLogLevels {
		tt, err := gtk.TextTagNew(loglevel)

		if err != nil {
			log.Fatal("Unable to populate tag table with color options:", err)
		}

		fmt.Println("Created tag table entry: ", loglevel)

		if loglevel == "all" {
			tt.SetProperty("foreground", colorLevelMap["default"])
		} else {
			tt.SetProperty("foreground", colorLevelMap[loglevel])
		}

		nbLabel, err := gtk.LabelNew(loglevel)

		if err != nil {
			log.Fatal("Unable to create notebook label:", err)
		}

		box, err := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 0)

		if err != nil {
			log.Fatal("Unable to create box:", err)
		}

		scrollbox, err := gtk.ScrolledWindowNew(nil, nil)

		if err != nil {
			log.Fatal("Unable to create scrolled window:", err)
		}

		scrollbox.Add(box)


		tv, err := gtk.TreeViewNew()

		if err != nil {
			log.Fatal("Unable to create treeview:", err)
		}

		tv.SetSizeRequest(300, 300)
		tv.SetHeadersClickable(true)

		cb, err := gtk.ButtonNewWithLabel("Clear buffer")

		if err != nil {
			log.Fatal("Unable to create button:", err)
		}

		box.Add(cb)
		box.Add(tv)

		tv.AppendColumn(createColumn("#", 0))
		tv.AppendColumn(createColumn("Date", 1))
		lcol := createColumn("Level", 2)
		tv.AppendColumn(lcol)

		if loglevel != "all" {
			lcol.SetVisible(false)
		}

		tv.AppendColumn(createColumn("Provider", 3))
		tv.AppendColumn(createColumn("Process", 4))
		tv.AppendColumn(createColumn("Line", 5))

		lcol = createColumn("OLine", 6)
		lcol.SetVisible(false)
		tv.AppendColumn(lcol)

		listStore := createLogListStore(true)

		tv.SetModel(listStore)

		lls := loglevel
		cb.Connect("clicked", func() {
			choice := promptChoice("Do you really want to clear the " + lls + " log buffer?")

			if choice != int(gtk.RESPONSE_YES) {
				return
			}

			fmt.Println(logBuffer)
			listStore.Clear()
			logBuffer[lls] = make([]logBuffered, 0)
		})

		tv.Connect("row-activated", func() {
			fmt.Println("DOUBLE CLICK")

			sel, err := tv.GetSelection()

			if err != nil {
				promptError("Unexpected error retrieving selection: "+err.Error())
				return
			}

			rows := sel.GetSelectedRows(listStore)
			// func (v *TreeSelection) GetSelected() (model ITreeModel, iter *TreeIter, ok bool)      ???
			fmt.Println("RETURNED ROWS: ", rows.Length())

			if rows.Length() > 0 {
				rdata := rows.NthData(0)

				lIndex, err := strconv.Atoi(rdata.(*gtk.TreePath).String())

				if err != nil {
					promptError("Unexpected error reading selection data: "+err.Error())
					return
				}


				path, err := gtk.TreePathNewFromString(fmt.Sprintf("%d", lIndex))

				if err != nil {
					promptError("Unexpected error reading data from selection: "+err.Error())
					return
				}

				iter, err := listStore.GetIter(path)

				if err != nil {
					promptError("Unexpected error looking up log entry: "+err.Error())
					return
				}

				val, err := listStore.GetValue(iter, 6)

				if err != nil {
					promptError("Unexpected error getting data from log entry: "+err.Error())
					return
				}

				sval, err := val.GetString()

				if err != nil {
					promptError("Unexpected error reading data from log entry: "+err.Error())
					return
				}

				fmt.Println("HEH: ", sval)
				promptInfo(sval)
			}

                })


		scrollbox.SetSizeRequest(600, 800)
		Notebook.AppendPage(scrollbox, nbLabel)

		newTab := sublogTabView{ loglevel, listStore }
		allTabs[loglevel] = newTab
	}

	setup_settings()
	mainWin.Add(Notebook)

	if userPrefs.Winheight > 0 && userPrefs.Winwidth > 0 {
		fmt.Printf("height was %d, width was %d\n", userPrefs.Winheight, userPrefs.Winwidth)
		mainWin.Resize(int(userPrefs.Winwidth), int(userPrefs.Winheight))
	} else {
		mainWin.SetDefaultSize(800, 600)
	}

	if userPrefs.Wintop > 0 && userPrefs.Winleft > 0 {
		mainWin.Move(int(userPrefs.Winleft), int(userPrefs.Wintop))
	}

	if len(userPrefs.Logfile) > 0 {
		fmt.Println("About to try to load log: " + userPrefs.Logfile)
		openOutLog(userPrefs.Logfile)
	}

	colScale.SetValue(float64(userPrefs.CollapseWin))

	mainWin.ShowAll()
	gtk.Main()      // GTK main loop; blocks until gtk.MainQuit() is run. 
}
