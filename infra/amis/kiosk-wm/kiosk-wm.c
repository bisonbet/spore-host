#include <X11/Xlib.h>
#include <X11/extensions/Xrandr.h>
#include <X11/extensions/XInput2.h>
#include <X11/Xatom.h>
#include <stdio.h>
#include <fcntl.h>
#include <unistd.h>
#include <sys/stat.h>
#include <sys/select.h>

#define ACTIVITY_FILE "/run/spore/x11-last-activity"

static int width, height;
static Display *dpy;
static Window root;
static Atom net_wm_state;
static Atom net_wm_state_max_vert;
static Atom net_wm_state_max_horz;
static Atom net_wm_state_fullscreen;
static Atom motif_hints;
static Atom wm_window_type;
static Atom wm_type_normal;
static Atom wm_type_dialog;
static Atom wm_transient_for_atom;

static void touch_activity(void) {
    mkdir("/run/spore", 0755);
    int fd = open(ACTIVITY_FILE, O_WRONLY | O_CREAT, 0644);
    if (fd >= 0) close(fd);
    utimes(ACTIVITY_FILE, NULL);
}

/* Return 1 if this is a main app window (not a dialog/internal window) */
static int is_main_window(Window win) {
    if (!win || win == root) return 0;

    /* Must have WM_CLASS */
    XClassHint hint;
    if (!XGetClassHint(dpy, win, &hint)) return 0;
    int skip = 0;
    if (hint.res_class) {
        if (strncmp(hint.res_class, "Dcv", 3) == 0) skip = 1;
    }
    if (hint.res_name) {
        if (strncmp(hint.res_name, "dcv", 3) == 0) skip = 1;
        XFree(hint.res_name);
    }
    if (hint.res_class) XFree(hint.res_class);
    if (skip) return 0;

    /* Skip transient (dialog) windows */
    Window transient = 0;
    XGetTransientForHint(dpy, win, &transient);
    if (transient && transient != root) return 0;

    return 1;
}

/* Ask metacity to maximize this window via EWMH (no direct resize) */
static void maximize_window(Window win) {
    if (!is_main_window(win)) return;

    /* Remove decorations */
    long hints[5] = { 2, 0, 0, 0, 0 };
    XChangeProperty(dpy, win, motif_hints, motif_hints, 32,
                    PropModeReplace, (unsigned char *)hints, 5);

    /* Send _NET_WM_STATE message to ask WM to maximize */
    XEvent ev = {0};
    ev.type = ClientMessage;
    ev.xclient.window = win;
    ev.xclient.message_type = net_wm_state;
    ev.xclient.format = 32;
    ev.xclient.data.l[0] = 1; /* _NET_WM_STATE_ADD */
    ev.xclient.data.l[1] = (long)net_wm_state_max_vert;
    ev.xclient.data.l[2] = (long)net_wm_state_max_horz;
    ev.xclient.data.l[3] = 1;
    XSendEvent(dpy, root, False,
               SubstructureNotifyMask | SubstructureRedirectMask, &ev);
    XFlush(dpy);
    fprintf(stderr, "kiosk-wm: maximize requested for window %lu\n", win);
}

static void maximize_all(void) {
    Window parent, *children;
    unsigned int n;
    if (!XQueryTree(dpy, root, &root, &parent, &children, &n)) return;
    int count = 0;
    for (unsigned int i = 0; i < n; i++) {
        if (is_main_window(children[i])) {
            maximize_window(children[i]);
            count++;
        }
    }
    if (children) XFree(children);
    if (count) fprintf(stderr, "kiosk-wm: maximized %d windows\n", count);
}

int main(void) {
    dpy = XOpenDisplay(NULL);
    if (!dpy) return 1;

    int screen = DefaultScreen(dpy);
    root = DefaultRootWindow(dpy);
    width  = DisplayWidth(dpy, screen);
    height = DisplayHeight(dpy, screen);
    fprintf(stderr, "kiosk-wm: %dx%d\n", width, height);

    int rr_base, rr_err;
    int has_rr = XRRQueryExtension(dpy, &rr_base, &rr_err);
    if (has_rr) XRRSelectInput(dpy, root, RRScreenChangeNotifyMask);

    XSelectInput(dpy, root, SubstructureNotifyMask);

    /* XInput2 for activity tracking */
    int xi_op, xi_ev, xi_err;
    if (XQueryExtension(dpy, "XInputExtension", &xi_op, &xi_ev, &xi_err)) {
        int major = 2, minor = 0;
        if (XIQueryVersion(dpy, &major, &minor) == Success) {
            XIEventMask mask;
            unsigned char bits[4] = {0};
            mask.deviceid = XIAllMasterDevices;
            mask.mask_len = sizeof(bits);
            mask.mask = bits;
            XISetMask(bits, XI_RawMotion);
            XISetMask(bits, XI_RawKeyPress);
            XISetMask(bits, XI_RawButtonPress);
            XISelectEvents(dpy, root, &mask, 1);
        }
    }

    motif_hints          = XInternAtom(dpy, "_MOTIF_WM_HINTS", False);
    net_wm_state         = XInternAtom(dpy, "_NET_WM_STATE", False);
    net_wm_state_max_vert = XInternAtom(dpy, "_NET_WM_STATE_MAXIMIZED_VERT", False);
    net_wm_state_max_horz = XInternAtom(dpy, "_NET_WM_STATE_MAXIMIZED_HORZ", False);
    net_wm_state_fullscreen = XInternAtom(dpy, "_NET_WM_STATE_FULLSCREEN", False);
    wm_window_type       = XInternAtom(dpy, "_NET_WM_WINDOW_TYPE", False);
    wm_type_normal       = XInternAtom(dpy, "_NET_WM_WINDOW_TYPE_NORMAL", False);
    wm_type_dialog       = XInternAtom(dpy, "_NET_WM_WINDOW_TYPE_DIALOG", False);
    wm_transient_for_atom = XInternAtom(dpy, "WM_TRANSIENT_FOR", False);

    maximize_all();
    touch_activity();

    int xfd = ConnectionNumber(dpy);

    for (;;) {
        fd_set fds;
        FD_ZERO(&fds);
        FD_SET(xfd, &fds);
        struct timeval tv = { 1, 0 };
        select(xfd + 1, &fds, NULL, NULL, &tv);

        /* Poll for display resize */
        int new_w = DisplayWidth(dpy, screen);
        int new_h = DisplayHeight(dpy, screen);
        if (new_w != width || new_h != height) {
            width = new_w; height = new_h;
            fprintf(stderr, "kiosk-wm: resized to %dx%d\n", width, height);
            maximize_all();
            touch_activity();
        }

        while (XPending(dpy)) {
            XEvent ev;
            XNextEvent(dpy, &ev);

            if (has_rr && ev.type == rr_base + RRScreenChangeNotify) {
                XRRUpdateConfiguration(&ev);
                width  = DisplayWidth(dpy, screen);
                height = DisplayHeight(dpy, screen);
                fprintf(stderr, "kiosk-wm: RandR resize to %dx%d\n", width, height);
                maximize_all();
                touch_activity();
                continue;
            }

            if (ev.type == GenericEvent) {
                if (XGetEventData(dpy, &ev.xcookie)) {
                    touch_activity();
                    XFreeEventData(dpy, &ev.xcookie);
                }
                continue;
            }

            Window win = 0;
            if (ev.type == MapNotify)         win = ev.xmap.window;
            else if (ev.type == CreateNotify) win = ev.xcreatewindow.window;

            if (win) maximize_window(win);
        }
    }
    return 0;
}
