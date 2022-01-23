//go:build linux
// +build linux

package linux

/*
#cgo linux pkg-config: gtk+-3.0 webkit2gtk-4.0

#include "gtk/gtk.h"
#include "webkit2/webkit2.h"
#include <stdio.h>
#include <limits.h>

static GtkWidget* GTKWIDGET(void *pointer) {
	return GTK_WIDGET(pointer);
}

static GtkWindow* GTKWINDOW(void *pointer) {
	return GTK_WINDOW(pointer);
}

static void SetMinSize(GtkWindow* window, int width, int height) {
	GdkGeometry size;
	size.min_height = height;
	size.min_width = width;
	gtk_window_set_geometry_hints(window, NULL, &size, GDK_HINT_MIN_SIZE);
}

static void SetMaxSize(GtkWindow* window, int width, int height) {
	GdkGeometry size;
	if( width == 0 ) {
		width = INT_MAX;
	}
	if( height == 0 ) {
		height = INT_MAX;
	}

	size.max_height = height;
	size.max_width = width;
	gtk_window_set_geometry_hints(window, NULL, &size, GDK_HINT_MAX_SIZE);
}

GdkRectangle getCurrentMonitorGeometry(GtkWindow *window) {
    // Get the monitor that the window is currently on
    GdkDisplay *display = gtk_widget_get_display(GTK_WIDGET(window));
    GdkWindow *gdk_window = gtk_widget_get_window(GTK_WIDGET(window));
    GdkMonitor *monitor = gdk_display_get_monitor_at_window (display, gdk_window);

    // Get the geometry of the monitor
    GdkRectangle result;
    gdk_monitor_get_geometry (monitor,&result);
    return result;
}

void SetPosition(GtkWindow *window, int x, int y) {
	GdkRectangle monitorDimensions = getCurrentMonitorGeometry(window);
	gtk_window_move(window, monitorDimensions.x + x, monitorDimensions.y + y);
}

void Center(GtkWindow *window)
{
    // Get the geometry of the monitor
    GdkRectangle m = getCurrentMonitorGeometry(window);

    // Get the window width/height
    int windowWidth, windowHeight;
    gtk_window_get_size(window, &windowWidth, &windowHeight);

	int newX = ((m.width - windowWidth) / 2) + m.x;
	int newY = ((m.height - windowHeight) / 2) + m.y;

    // Place the window at the center of the monitor
    gtk_window_move(window, newX, newY);
}

int IsFullscreen(GtkWidget *widget) {
	GdkWindow *gdkwindow = gtk_widget_get_window(widget);
	GdkWindowState state = gdk_window_get_state(GDK_WINDOW(gdkwindow));
	return state & GDK_WINDOW_STATE_FULLSCREEN == GDK_WINDOW_STATE_FULLSCREEN;
}

extern void processMessage(char*);

static void sendMessageToBackend(WebKitUserContentManager *contentManager,
                                 WebKitJavascriptResult *result,
                                 void* data)
{
#if WEBKIT_MAJOR_VERSION >= 2 && WEBKIT_MINOR_VERSION >= 22
    JSCValue *value = webkit_javascript_result_get_js_value(result);
    char *message = jsc_value_to_string(value);
#else
    JSGlobalContextRef context = webkit_javascript_result_get_global_context(result);
    JSValueRef value = webkit_javascript_result_get_value(result);
    JSStringRef js = JSValueToStringCopy(context, value, NULL);
    size_t messageSize = JSStringGetMaximumUTF8CStringSize(js);
    char *message = g_new(char, messageSize);
    JSStringGetUTF8CString(js, message, messageSize);
    JSStringRelease(js);
#endif
    processMessage(message);
    g_free(message);
}

ulong setupInvokeSignal(void* contentManager) {
	return g_signal_connect((WebKitUserContentManager*)contentManager, "script-message-received::external", G_CALLBACK(sendMessageToBackend), NULL);
}

// These are the x,y & time of the last mouse down event
// It's used for window dragging
float xroot = 0.0f;
float yroot = 0.0f;
int dragTime = -1;

gboolean buttonPress(GtkWidget *widget, GdkEventButton *event, void* dummy)
{
	if( event == NULL ) {
		xroot = yroot = 0.0f;
		dragTime = -1;
		return FALSE;
	}
	if (event->type == GDK_BUTTON_PRESS && event->button == 1)
    {
        xroot = event->x_root;
        yroot = event->y_root;
        dragTime = event->time;
    }
    return FALSE;
}

gboolean buttonRelease(GtkWidget *widget, GdkEventButton *event, void* dummy)
{
    if (event == NULL || (event->type == GDK_BUTTON_RELEASE && event->button == 1))
    {
		xroot = yroot = 0.0f;
		dragTime = -1;
    }
    return FALSE;
}

void connectButtons(void* webview) {
	g_signal_connect(WEBKIT_WEB_VIEW(webview), "button-press-event", G_CALLBACK(buttonPress), NULL);
	g_signal_connect(WEBKIT_WEB_VIEW(webview), "button-release-event", G_CALLBACK(buttonRelease), NULL);
}

extern void processURLRequest(WebKitURISchemeRequest *request);

// This is called when the close button on the window is pressed
gboolean close_button_pressed(GtkWidget *widget, GdkEvent *event, void* data)
{
   	processMessage("Q");
    return FALSE;
}

GtkWidget* setupWebview(void* contentManager, GtkWindow* window, int hideWindowOnClose) {
	GtkWidget* webview = webkit_web_view_new_with_user_content_manager((WebKitUserContentManager*)contentManager);
	gtk_container_add(GTK_CONTAINER(window), webview);
	WebKitWebContext *context = webkit_web_context_get_default();
	webkit_web_context_register_uri_scheme(context, "wails", (WebKitURISchemeRequestCallback)processURLRequest, NULL, NULL);
	//g_signal_connect(G_OBJECT(webview), "load-changed", G_CALLBACK(webview_load_changed_cb), NULL);
	if (hideWindowOnClose) {
		g_signal_connect(GTK_WIDGET(window), "delete-event", G_CALLBACK(gtk_widget_hide_on_delete), NULL);
	} else {
		g_signal_connect(GTK_WIDGET(window), "delete-event", G_CALLBACK(close_button_pressed), NULL);
	}
	return webview;
}

void devtoolsEnabled(void* webview, int enabled) {
	WebKitSettings *settings = webkit_web_view_get_settings(WEBKIT_WEB_VIEW(webview));
	gboolean genabled = enabled == 1 ? true : false;
	webkit_settings_set_enable_developer_extras(settings, genabled);
}

void loadIndex(void* webview) {
	webkit_web_view_load_uri(WEBKIT_WEB_VIEW(webview), "wails:///");
}

static void startDrag(void *webview, GtkWindow* mainwindow)
{
    // Ignore non-toplevel widgets
    GtkWidget *window = gtk_widget_get_toplevel(GTK_WIDGET(webview));
    if (!GTK_IS_WINDOW(window)) return;

    gtk_window_begin_move_drag(mainwindow, 1, xroot, yroot, dragTime);
}

typedef struct JSCallback {
    void* webview;
    char* script;
} JSCallback;

int executeJS(gpointer data) {
    struct JSCallback *js = data;
    webkit_web_view_run_javascript(js->webview, js->script, NULL, NULL, NULL);
    free(js->script);
    return G_SOURCE_REMOVE;
}

void ExecuteOnMainThread(JSCallback* jscallback) {
    g_idle_add((GSourceFunc)executeJS, (gpointer)jscallback);
}
*/
import "C"
import (
	"github.com/wailsapp/wails/v2/pkg/options"
	"unsafe"
)

func gtkBool(input bool) C.gboolean {
	if input {
		return C.gboolean(1)
	}
	return C.gboolean(0)
}

type Window struct {
	appoptions     *options.App
	debug          bool
	gtkWindow      unsafe.Pointer
	contentManager unsafe.Pointer
	webview        unsafe.Pointer
}

func bool2Cint(value bool) C.int {
	if value {
		return C.int(1)
	}
	return C.int(0)
}

func NewWindow(appoptions *options.App, debug bool) *Window {

	result := &Window{
		appoptions: appoptions,
		debug:      debug,
	}

	gtkWindow := C.gtk_window_new(C.GTK_WINDOW_TOPLEVEL)
	C.g_object_ref_sink(C.gpointer(gtkWindow))
	result.gtkWindow = unsafe.Pointer(gtkWindow)

	result.contentManager = unsafe.Pointer(C.webkit_user_content_manager_new())
	external := C.CString("external")
	defer C.free(unsafe.Pointer(external))
	C.webkit_user_content_manager_register_script_message_handler(result.cWebKitUserContentManager(), external)
	C.setupInvokeSignal(result.contentManager)

	webview := C.setupWebview(result.contentManager, result.asGTKWindow(), bool2Cint(appoptions.HideWindowOnClose))
	result.webview = unsafe.Pointer(webview)
	buttonPressedName := C.CString("button-press-event")
	defer C.free(unsafe.Pointer(buttonPressedName))
	C.connectButtons(unsafe.Pointer(webview))

	if debug {
		C.devtoolsEnabled(unsafe.Pointer(webview), C.int(1))
	}

	// Setup window
	result.SetKeepAbove(appoptions.AlwaysOnTop)
	result.SetResizable(!appoptions.DisableResize)
	result.SetSize(appoptions.Width, appoptions.Height)
	result.SetDecorated(!appoptions.Frameless)
	result.SetTitle(appoptions.Title)
	result.SetMinSize(appoptions.MinWidth, appoptions.MinHeight)
	result.SetMaxSize(appoptions.MaxWidth, appoptions.MaxHeight)

	return result
}

func (w *Window) asGTKWidget() *C.GtkWidget {
	return C.GTKWIDGET(w.gtkWindow)
}

func (w *Window) asGTKWindow() *C.GtkWindow {
	return C.GTKWINDOW(w.gtkWindow)
}

func (w *Window) cWebKitUserContentManager() *C.WebKitUserContentManager {
	return (*C.WebKitUserContentManager)(w.contentManager)
}

func (w *Window) Fullscreen() {
	C.gtk_window_fullscreen(w.asGTKWindow())
}

func (w *Window) UnFullscreen() {
	C.gtk_window_unfullscreen(w.asGTKWindow())
}

func (w *Window) Destroy() {
	C.g_object_unref(C.gpointer(w.gtkWindow))
	C.gtk_widget_destroy(w.asGTKWidget())
}

func (w *Window) Close() {
	C.gtk_window_close(w.asGTKWindow())
}

func (w *Window) Center() {
	C.Center(w.asGTKWindow())
}

func (w *Window) SetPos(x int, y int) {
	cX := C.int(x)
	cY := C.int(y)
	C.gtk_window_move(w.asGTKWindow(), cX, cY)
}

func (w *Window) Size() (int, int) {
	var width, height C.int
	C.gtk_window_get_size(w.asGTKWindow(), &width, &height)
	return int(width), int(height)
}

func (w *Window) Pos() (int, int) {
	var width, height C.int
	C.gtk_window_get_position(w.asGTKWindow(), &width, &height)
	return int(width), int(height)
}

func (w *Window) SetMaxSize(maxWidth int, maxHeight int) {
	C.SetMaxSize(w.asGTKWindow(), C.int(maxWidth), C.int(maxHeight))
}

func (w *Window) SetMinSize(minWidth int, minHeight int) {
	C.SetMinSize(w.asGTKWindow(), C.int(minWidth), C.int(minHeight))
}

func (w *Window) Show() {
	C.gtk_widget_show(w.asGTKWidget())
}

func (w *Window) Hide() {
	C.gtk_widget_hide(w.asGTKWidget())
}

func (w *Window) Maximise() {
	C.gtk_window_maximize(w.asGTKWindow())
}

func (w *Window) UnMaximise() {
	C.gtk_window_unmaximize(w.asGTKWindow())
}

func (w *Window) Minimise() {
	C.gtk_window_iconify(w.asGTKWindow())
}

func (w *Window) UnMinimise() {
	C.gtk_window_present(w.asGTKWindow())
}

func (w *Window) IsFullScreen() bool {
	result := C.IsFullscreen(w.asGTKWidget())
	if result == 1 {
		return true
	}
	return false
}

func (w *Window) SetRGBA(r uint8, g uint8, b uint8, a uint8) {
	//C.SetRGBA(w.context, C.int(r), C.int(g), C.int(b), C.int(a))
}

//func (w *Window) SetApplicationMenu(inMenu *menu.Menu) {
//	//mainMenu := NewNSMenu(w.context, "")
//	//processMenu(mainMenu, inMenu)
//	//C.SetAsApplicationMenu(w.context, mainMenu.nsmenu)
//}

func (w *Window) UpdateApplicationMenu() {
	//C.UpdateApplicationMenu(w.context)
}

func (w *Window) Run() {
	C.loadIndex(w.webview)
	C.gtk_widget_show_all(w.asGTKWidget())
	w.Center()
	switch w.appoptions.WindowStartState {
	case options.Fullscreen:
		w.Fullscreen()
	case options.Minimised:
		w.Minimise()
	case options.Maximised:
		w.Maximise()
	}
	C.gtk_main()
	w.Destroy()
}

func (w *Window) SetKeepAbove(top bool) {
	C.gtk_window_set_keep_above(w.asGTKWindow(), gtkBool(top))
}

func (w *Window) SetResizable(resizable bool) {
	C.gtk_window_set_resizable(w.asGTKWindow(), gtkBool(resizable))
}

func (w *Window) SetSize(width int, height int) {
	C.gtk_window_resize(w.asGTKWindow(), C.gint(width), C.gint(height))
}

func (w *Window) SetDecorated(frameless bool) {
	C.gtk_window_set_decorated(w.asGTKWindow(), gtkBool(frameless))
}

func (w *Window) SetTitle(title string) {
	cTitle := C.CString(title)
	defer C.free(unsafe.Pointer(cTitle))
	C.gtk_window_set_title(w.asGTKWindow(), cTitle)
}

func (w *Window) ExecJS(js string) {
	jscallback := C.JSCallback{
		webview: w.webview,
		script: C.CString(js),
	}
	C.ExecuteOnMainThread(&jscallback)
}

func (w *Window) StartDrag() {
	C.startDrag(w.webview, w.asGTKWindow())
}

func (w *Window) Quit() {
	C.gtk_main_quit()
}
