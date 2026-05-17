#include <X11/Xlib.h>
#include <stdio.h>

int main() {
    Display *display = XOpenDisplay(0x0);
    if (!display) return 1;

    int screen = DefaultScreen(display);
    Window root = DefaultRootWindow(display);

    /* Read actual display dimensions rather than hardcoding 1920x1080 */
    int width  = DisplayWidth(display, screen);
    int height = DisplayHeight(display, screen);

    fprintf(stderr, "kiosk-wm: %dx%d\n", width, height);

    /* SubstructureRedirectMask lets us intercept MapRequest (new windows) */
    XSelectInput(display, root, SubstructureNotifyMask | SubstructureRedirectMask);

    /* Atom for removing window decorations (_MOTIF_WM_HINTS) */
    Atom motif_hints = XInternAtom(display, "_MOTIF_WM_HINTS", False);

    for (;;) {
        XEvent ev;
        XNextEvent(display, &ev);

        Window win = 0;

        if (ev.type == MapRequest) {
            /* New window wants to be shown — map it then force fullscreen */
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
            /* Strip all window decorations (title bar, border) */
            long hints[5] = { 2, 0, 0, 0, 0 }; /* flags=decorations, decorations=0 */
            XChangeProperty(display, win, motif_hints, motif_hints, 32,
                            PropModeReplace, (unsigned char *)hints, 5);

            /* Force window to fill the entire display */
            XMoveResizeWindow(display, win, 0, 0, width, height);
        }
    }

    return 0;
}
