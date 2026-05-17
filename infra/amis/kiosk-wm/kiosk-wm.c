#include <X11/Xlib.h>
#include <X11/extensions/Xrandr.h>
#include <stdio.h>
#include <fcntl.h>
#include <unistd.h>
#include <sys/stat.h>

#define ACTIVITY_FILE "/run/spore/x11-last-activity"

static int width, height;
static Display *display;
static Window root;
static Atom motif_hints;

static void record_activity(void) {
    mkdir("/run/spore", 0755);
    int fd = open(ACTIVITY_FILE, O_WRONLY | O_CREAT, 0644);
    if (fd >= 0) close(fd);
    utimes(ACTIVITY_FILE, NULL);
}

static void fill_window(Window win) {
    if (!win || win == root) return;
    long hints[5] = { 2, 0, 0, 0, 0 };
    XChangeProperty(display, win, motif_hints, motif_hints, 32,
                    PropModeReplace, (unsigned char *)hints, 5);
    XMoveResizeWindow(display, win, 0, 0, width, height);
    XRaiseWindow(display, win);
}

static void fill_all_windows(void) {
    Window parent, *children;
    unsigned int n;
    if (!XQueryTree(display, root, &root, &parent, &children, &n)) return;
    for (unsigned int i = 0; i < n; i++) fill_window(children[i]);
    if (children) XFree(children);
    fprintf(stderr, "kiosk-wm: filled %u windows at %dx%d\n", n, width, height);
}

int main() {
    display = XOpenDisplay(NULL);
    if (!display) return 1;

    int screen = DefaultScreen(display);
    root = DefaultRootWindow(display);
    width  = DisplayWidth(display, screen);
    height = DisplayHeight(display, screen);
    fprintf(stderr, "kiosk-wm: %dx%d, activity: %s\n", width, height, ACTIVITY_FILE);

    int rr_base, rr_err;
    int has_rr = XRRQueryExtension(display, &rr_base, &rr_err);
    if (has_rr) XRRSelectInput(display, root, RRScreenChangeNotifyMask);

    /* Use SubstructureNotify (not Redirect) — windows map themselves,
     * we just reposition them. Redirect caused MapRequest to block. */
    XSelectInput(display, root,
        SubstructureNotifyMask |
        KeyPressMask | ButtonPressMask | PointerMotionMask);

    XGrabKey(display, AnyKey, AnyModifier, root, False, GrabModeAsync, GrabModeAsync);
    XGrabButton(display, AnyButton, AnyModifier, root, False,
                ButtonPressMask, GrabModeAsync, GrabModeAsync, None, None);

    motif_hints = XInternAtom(display, "_MOTIF_WM_HINTS", False);

    fill_all_windows();
    record_activity();

    for (;;) {
        XEvent ev;
        XNextEvent(display, &ev);

        if (has_rr && ev.type == rr_base + RRScreenChangeNotify) {
            XRRUpdateConfiguration(&ev);
            width  = DisplayWidth(display, screen);
            height = DisplayHeight(display, screen);
            fprintf(stderr, "kiosk-wm: resized to %dx%d\n", width, height);
            fill_all_windows();
            continue;
        }

        if (ev.type == KeyPress || ev.type == ButtonPress || ev.type == MotionNotify) {
            record_activity();
            if (ev.type == ButtonPress) XAllowEvents(display, ReplayPointer, CurrentTime);
            continue;
        }

        /* A window was mapped or configured — fill it */
        Window win = 0;
        if (ev.type == MapNotify)       win = ev.xmap.window;
        else if (ev.type == CreateNotify)  win = ev.xcreatewindow.window;
        else if (ev.type == ConfigureNotify) {
            XConfigureEvent ce = ev.xconfigure;
            if (ce.x != 0 || ce.y != 0 || ce.width != width || ce.height != height)
                win = ce.window;
        }

        if (win && win != root) fill_window(win);
    }
    return 0;
}
