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
)


type sublogTabView struct {
	LogLevel string
	TV *gtk.TextView
	TVBuffer *gtk.TextBuffer
}

type logSuppression struct {
	Description string
	Wildcard string
	Metadata map[string]string
	Count int
}

type logBuffered struct {
	Line string
	Metadata map[string]string
	Timestamps []time.Time
	LineTV int
}


var allTabs map[string]sublogTabView
var allSuppressions []logSuppression
var logBuffer = map[string][]logBuffered { "critical": {}, "alert": {}, "default": {} }
var Notebook *gtk.Notebook
var globalLS *gtk.ListStore

var allLogLevels = []string { "critical", "alert", "default", "all" }

var colorLevelMap = map[string]string{
	"critical": "red",
	"alert": "orange",
	"default": "black",
}


func init_log_buffer() {
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

func buffer_line(loglevel, line string, metadata map[string]string) (int, logBuffered) {
	found := -1
	now := time.Now()

	fmt.Printf("looking in section %s / len = %d\n", loglevel, len(logBuffer[loglevel]))

	for i := 0; i < len(logBuffer[loglevel]); i++ {

		if logBuffer[loglevel][i].Line == line {
			found = i
			break
		}

	}

	if found >= 0 {
		logBuffer[loglevel][found].Timestamps = append(logBuffer[loglevel][found].Timestamps, now)
		return len(logBuffer[loglevel][found].Timestamps), logBuffer[loglevel][found]
	}

	lineno := allTabs[loglevel].TVBuffer.GetLineCount() - 1

	fmt.Println("_____________ lineno = ", lineno)
	newbuf := logBuffered { line, metadata, []time.Time{ now }, lineno }
	logBuffer[loglevel] = append(logBuffer[loglevel], newbuf)
	return 0, newbuf
}

func appendLogToTVB(tvb *gtk.TextBuffer, line, loglevel string, timestamp int64) {
	tss := time.Unix(timestamp / 1000000000, timestamp % 1000000000).Format(time.UnixDate)
	iend := tvb.GetEndIter()
	lastOffset := iend.GetOffset()
	tvb.Insert(iend, tss)
	xistart, xiend := tvb.GetBounds()
	xistart = tvb.GetIterAtOffset(lastOffset)
	tvb.ApplyTagByName("underline", xistart, xiend)

	iend = tvb.GetEndIter()
	lastOffset = iend.GetOffset()
	tvb.Insert(iend, " "+line+"\n")
	xistart, xiend = tvb.GetBounds()
	xistart = tvb.GetIterAtOffset(lastOffset)
//	tvb.ApplyTag(colorTag, xistart, xiend)
	tvb.ApplyTagByName(loglevel, xistart, xiend)
}

func appendLogLine(line, loglevel string, timestamp int64, section, all bool) {
	thisTab := allTabs[loglevel]

	if thisTab.TVBuffer == nil {
		fmt.Println("Got logging data but application was not initialized yet...")
		return
	}

	fmt.Println("heh and then logged something.")

//	colorTag, err := tvTagTable.Lookup(loglevel)

	if section {
		appendLogToTVB(thisTab.TVBuffer, line, loglevel, timestamp)
	}

	if all {
		appendLogToTVB(allTabs["all"].TVBuffer, "["+loglevel+"] "+line, loglevel, timestamp)
	}

}

func guiLog(data slmData) {
	fmt.Printf("XXX: loglevel = %s, eventid = %s\n", data.LogLevel, data.EventID)
	suppressed := false

	for i := 0; i < len(allSuppressions); i++ {

		if len(allSuppressions[i].Wildcard) == 0 {
			fmt.Println("XXX: possible regex match: ", allSuppressions[i].Description)
//			allSuppressions[i].Count += 1
//			suppressed = true
		} else {
			matched, err := regexp.MatchString(allSuppressions[i].Wildcard, data.LogLine)

			if err == nil && matched {
				fmt.Println("XXX: might wildcard against: ", allSuppressions[i].Description)
				allSuppressions[i].Count++
				suppressed = true
				update_suppression_count(i, allSuppressions[i].Count)
			}

		}

		fmt.Println("suppressions count for ", allSuppressions[i].Description, " = ", allSuppressions[i].Count)
	}

//	Metadata map[string]string

	if suppressed {
		fmt.Println("*** WAS SUPPRESSED: ", data.LogLine)
		return
	}

	nbuf, bufentry := buffer_line(data.LogLevel, data.LogLine, data.Metadata)
	fmt.Println("---------- nbuf = ", nbuf)

	if nbuf > 0 {
		fmt.Println("+++++++++++++++ should overwrite line: ", bufentry.LineTV)

		starter := allTabs[data.LogLevel].TVBuffer.GetStartIter()
		starter.SetLine(bufentry.LineTV)
		off1 := starter.GetOffset()

		if nbuf > 2 {
			inssPrev := fmt.Sprintf("[%dx] ", nbuf-1)
			ender := allTabs[data.LogLevel].TVBuffer.GetIterAtOffset(off1 + len(inssPrev))
			allTabs[data.LogLevel].TVBuffer.Delete(starter, ender)
			// ???
			starter = allTabs[data.LogLevel].TVBuffer.GetStartIter()
			starter.SetLine(bufentry.LineTV)
		}

		inss := fmt.Sprintf("[%dx] ", nbuf)
		allTabs[data.LogLevel].TVBuffer.Insert(starter, inss)

		starter = allTabs[data.LogLevel].TVBuffer.GetStartIter()
		starter.SetLine(bufentry.LineTV)
		ender := allTabs[data.LogLevel].TVBuffer.GetIterAtOffset(starter.GetOffset() + len(inss) - 1)
//		_, ender = allTabs[data.LogLevel].TVBuffer.GetBounds()
		allTabs[data.LogLevel].TVBuffer.ApplyTagByName("bold", starter, ender)
		allTabs[data.LogLevel].TVBuffer.ApplyTagByName("underline", starter, ender)
		appendLogLine(data.LogLine, data.LogLevel, data.Timestamp, false, true)
	} else {
		appendLogLine(data.LogLine, data.LogLevel, data.Timestamp, true, true)
	}
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
			fmt.Println("YYY appending: metadata name = ", i)
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

	return column
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

func addRow(listStore *gtk.ListStore, description, wildcard string, nadded int) {
	iter := listStore.Append()

	colVals := make([]interface{}, nadded+3)
	colVals[0] = 0
	colVals[1] = description
	colVals[2] = wildcard

	for n := 0; n < nadded; n++ {
		colVals[n+3] = "a"
	}

	colNums := make([]int, nadded+3)

	for n := 0; n < nadded+3; n++ {
		colNums[n] = n
	}


//	err := listStore.Set(iter, []int{0, 1, 2, 3, 4, 5}, []interface{}{0, description, wildcard, "a", "b", "c"})
	err := listStore.Set(iter, colNums, colVals)

	if err != nil {
		log.Fatal("Unable to add row:", err)
	}

}

func update_suppression_count(rownum, val int) {
	listStore := globalLS

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

	box.Add(scrollbox)
	scrollbox.SetSizeRequest(600, 800)

	supPath := "suppressions.json"
	success := loadSuppressions(supPath)

	var allMetadata []string

	tv, err := gtk.TreeViewNew()

	if err != nil {
		log.Fatal("Unable to create treeview:", err)
	}

	scrollbox.Add(tv)

	if success {
/*		lString := fmt.Sprintf("Loaded a total of %d suppressions from file: %s\n", len(allSuppressions), supPath)
		sLabel, err := gtk.LabelNew(lString)

		if err != nil {
			log.Fatal("Unable to create notebook label:", err)
		} */
		tv.AppendColumn(createColumn("#", 0))
		tv.AppendColumn(createColumn("Description", 1))
		tv.AppendColumn(createColumn("Wildcard", 2))

		listStore := createListStore(3)
		globalLS = listStore

		tv.SetModel(listStore)

		for n := 0; n < len(allSuppressions); n++ {
			allMetadata = add_all_unique_meta_fields(allMetadata, allSuppressions[n].Metadata)
			sort.Sort(sortStrings(allMetadata))
		}

		fmt.Printf("XXX: got %d unique metadata fields\n", len(allMetadata))
		for x := 0; x < len(allMetadata); x++ {
			fmt.Printf("XXX: %s\n", allMetadata[x])
			tv.AppendColumn(createColumn(allMetadata[x], x+3))
		}

		for n := 0; n < len(allSuppressions); n++ {
			addRow(listStore, allSuppressions[n].Description, allSuppressions[n].Wildcard, 3)
		}

	}

	Notebook.AppendPage(box, hLabel)
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
	return ulTT
}

func gui_main() {
	gtk.Init(nil)

	// Create a new toplevel window, set its title, and connect it to the "destroy" signal to exit the GTK main loop when it is destroyed.
	win, err := gtk.WindowNew(gtk.WINDOW_TOPLEVEL)

	if err != nil {
		log.Fatal("Unable to create window:", err)
	}

	win.SetTitle("Subgraph Log Monitor")

	win.Connect("destroy", func() {
		fmt.Println("Shutting down...")
	        gtk.MainQuit()
	})

	win.SetPosition(gtk.WIN_POS_CENTER)

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

		newTV, err := gtk.TextViewNew()

		if err != nil {
			log.Fatal("Unable to create TextView:", err)
		}

		newTVBuffer, err := newTV.GetBuffer()

		if err != nil {
			log.Fatal("Unable to get buffer:", err)
		}

		newTVBuffer.SetText("Initializing logging for severity level " + loglevel + "...\n\n")
		newTV.SetEditable(false)
		newTV.SetWrapMode(gtk.WRAP_WORD)

		tvTagTable, err := newTVBuffer.GetTagTable()

		if err != nil {
			log.Fatal("Unable to retrieve tag table:", err)
		}

		tvTagTable.Add(tt)
		tvTagTable.Add(get_bold_texttag())
		tvTagTable.Add(get_underline_texttag())

		defaultTextProvider, err := gtk.CssProviderNew()

		if err != nil {
			log.Fatal("Unable to create CSS provider:", err)
		}

		defaultTextProvider.LoadFromData("textview { font: 14px serif;} textview text { color: black;}")
		redStyleContext, err := newTV.GetStyleContext()
		redStyleContext.AddProvider(defaultTextProvider, gtk.STYLE_PROVIDER_PRIORITY_APPLICATION)

		newTV.SetLeftMargin(10)
		newTV.SetBorderWindowSize(gtk.TEXT_WINDOW_TOP, 10)

		box, err := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 0)

		if err != nil {
			log.Fatal("Unable to create box:", err)
		}

		scrollbox, err := gtk.ScrolledWindowNew(nil, nil)

		if err != nil {
			log.Fatal("Unable to create scrolled window:", err)
		}

		box.Add(scrollbox)
		scrollbox.Add(newTV)
		scrollbox.SetSizeRequest(600, 800)
		Notebook.AppendPage(box, nbLabel)

		newTab := sublogTabView{ loglevel, newTV, newTVBuffer }
		allTabs[loglevel] = newTab
	}

	allTVTagTable, err := allTabs["all"].TVBuffer.GetTagTable()

	if err != nil {
		log.Fatal("Unable to retrieve tag table of all logs:", err)
	}

	for _, loglevel := range allLogLevels {

		if loglevel == "all" {
			continue
		}

		allTT, err := gtk.TextTagNew(loglevel)

		if err != nil {
			log.Fatal("Unable to populate all logs tag table with color options:", err)
		}

		allTT.SetProperty("foreground", colorLevelMap[loglevel])
		allTVTagTable.Add(allTT)
	}

	setup_settings()
	win.Add(Notebook)


	win.SetDefaultSize(800, 600)
	win.ShowAll()
	gtk.Main()      // GTK main loop; blocks until gtk.MainQuit() is run. 
}
