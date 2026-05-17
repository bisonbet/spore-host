#include <X11/Xlib.h>
#include <stdio.h>
#include <fcntl.h>
#include <unistd.h>
#include <time.h>
#include <sys/stat.h>

#define ACTIVITY_FILE "/run/spore/x11-last-activity"

/* Touch the activity file to record X11 user activity (mouse/keyboard).
 * spored reads this file's mtime to determine DCV idle state. */
static void record_activity(void) {
    /* Ensure directory exists */
    mkdir("/run/spore", 0755);
    /* Update mtime — create if missing */
    int fd = open(ACTIVITY_FILE, O_WRONLY | O_CREAT, 0644);
    if (fd >= 0) {
        close(fd);
    }
    /* futimens would be cleaner but this works: open + close updates mtime */
    utimes(ACTIVITY_FILE, NULL);
}

int main() {
    Display *display = XOpenDisplay(0x0);
    if (!display) return 1;

    int screen = DefaultScreen(display);
    Window root = DefaultRootWindow(display);

    int width  = DisplayWidth(display, screen);
    int height = DisplayHeight(display, screen);

    fprintf(stderr, "kiosk-wm: %dx%d, activity file: %s\n",
            width, height, ACTIVITY_FILE);

    /* SubstructureRedirect to intercept new windows.
     * KeyPress + ButtonPress on root to detect user activity. */
    XSelectInput(display, root,
        SubstructureNotifyMask |
        SubstructureRedirectMask |
        KeyPressMask |
        ButtonPressMask |
        PointerMotionMask);

    /* Allow key events to be received on root (some WMs grab these) */
    XGrabKey(display, AnyKey, AnyModifier, root, False,
             GrabModeAsync, GrabModeAsync);
    XGrabButton(display, AnyButton, AnyModifier, root, False,
                ButtonPressMask, GrabModeAsync, GrabModeAsync, None, None);

    /* Atom for removing window decorations (_MOTIF_WM_HINTS) */
    Atom motif_hints = XInternAtom(display, "_MOTIF_WM_HINTS", False);

    /* Record initial activity so idle timer starts from now */
    record_activity();

    for (;;) {
        XEvent ev;
        XNextEvent(display, &ev);

        /* User activity events — update the activity timestamp */
        if (ev.type == KeyPress || ev.type == ButtonPress ||
            ev.type == MotionNotify) {
            record_activity();
            continue;
        }

        Window win = 0;

        if (ev.type == MapRequest) {
            win = ev.xmaprequest.window;
            XMapWindow(display, win);
        } else if (ev.type == CreateNotify) {
            win = ev.xcreatewindow.window;
        } else if (ev.type == ConfigureNotify) {
            XConfigureEvent ce = ev.xconfigure;
            if (ce.x != 0 || ce.y != 0 ||
                ce.width != width || ce.height != height) {
                win = ce.window;
            }
        } else if (ev.type == ConfigureRequest) {
            win = ev.xconfigurerequest.window;
        }

        if (win && win != root) {
            long hints[5] = { 2, 0, 0, 0, 0 };
            XChangeProperty(display, win, motif_hints, motif_hints, 32,
                            PropModeReplace, (unsigned char *)hints, 5);
            XMoveResizeWindow(display, win, 0, 0, width, height);
        }
    }

    return 0;
}
