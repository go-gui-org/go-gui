package com.example.androiddemo

import android.app.Activity
import android.os.Build
import android.view.View
import android.view.WindowInsets
import android.view.WindowInsetsController
import android.view.WindowManager
import androidapp.Androidapp

/**
 * Fullscreen for go-gui (issue #1171). Window.SetFullscreen leaves a
 * request that GoGuiRenderer polls with Androidapp.pendingFullscreen and
 * hands to [set] on the UI thread. The answer goes back through
 * Androidapp.fullscreenChanged and feeds Window.IsFullscreen.
 *
 * Fullscreen hides the status and navigation bars; a swipe from an edge
 * shows them for a moment. While fullscreen the view also reaches into
 * the display cutout (Android 9 to 14) and under the bars (Android 11 to
 * 14). Leaving undoes it all and brings back the bars that showed
 * before, so outside fullscreen the layout is the theme's own. Android
 * 15 and later draw edge to edge anyway, so only the bars change there.
 *
 * On Android 9 a theme that keeps the status bar leaves a strip over
 * the cutout in fullscreen (Android 10 is untested). The demo's
 * Theme.NoTitleBar.Fullscreen does not.
 *
 * Below Android 11 the system clears the hiding flags by itself, when
 * the app comes back to the front or the soft keyboard opens, so they
 * are applied again then.
 *
 * IsFullscreen is true while fullscreen is asked for and the activity
 * has the screen to itself: in multi-window mode it is false.
 *
 * A host that draws edge to edge itself on Android 11 to 14 keeps
 * setDecorFitsSystemWindows(false) on leaving instead of the true here.
 *
 * All methods run on the UI thread.
 */
class GoGuiFullscreen(
    private val activity: Activity,
    private val glView: GoGuiGLSurfaceView
) {
    private val sdk = Build.VERSION.SDK_INT
    private val window get() = activity.window

    private var want = false
    private var baseCutoutMode = 0
    // The bars as they were before entering, for leaving.
    private var savedUiVisibility = 0
    private var savedStatusVisible = true
    private var savedNavVisible = true
    private var savedBehavior = 0

    init {
        if (sdk >= 28) {
            baseCutoutMode = window.attributes.layoutInDisplayCutoutMode
        }
        if (sdk < 30) {
            @Suppress("DEPRECATION")
            window.decorView.setOnSystemUiVisibilityChangeListener { vis ->
                @Suppress("DEPRECATION")
                val shown = (vis and View.SYSTEM_UI_FLAG_FULLSCREEN) == 0
                if (want && shown) {
                    window.decorView.post { if (want) hideBars() }
                }
            }
        }
        // A new activity starts out of fullscreen; say so, in case Go
        // still holds the state of the one before.
        report()
    }

    /** Enters or leaves fullscreen. */
    fun set(on: Boolean) {
        if (on != want) {
            want = on
            if (on) saveBars()
            setFill(on)
            if (on) hideBars() else restoreBars()
        }
        report()
    }

    /** Call from Activity.onWindowFocusChanged. */
    fun onWindowFocusChanged(hasFocus: Boolean) {
        if (hasFocus && want && sdk < 30) hideBars()
        report()
    }

    /** Call from Activity.onMultiWindowModeChanged. */
    fun onMultiWindowModeChanged() = report()

    private fun saveBars() {
        if (sdk >= 30) {
            val i = window.decorView.rootWindowInsets
            savedStatusVisible =
                i?.isVisible(WindowInsets.Type.statusBars()) ?: true
            savedNavVisible =
                i?.isVisible(WindowInsets.Type.navigationBars()) ?: true
            savedBehavior = window.insetsController?.systemBarsBehavior ?: 0
        } else {
            @Suppress("DEPRECATION")
            savedUiVisibility = window.decorView.systemUiVisibility
        }
    }

    // setFill lets the view reach into the cutout and under the bars
    // while fullscreen, and puts the theme's settings back after.
    private fun setFill(on: Boolean) {
        if (sdk >= 35) return
        if (sdk >= 28) {
            val a = window.attributes
            a.layoutInDisplayCutoutMode = if (on) {
                WindowManager.LayoutParams.LAYOUT_IN_DISPLAY_CUTOUT_MODE_SHORT_EDGES
            } else {
                baseCutoutMode
            }
            window.attributes = a
        }
        if (sdk >= 30) window.setDecorFitsSystemWindows(!on)
    }

    private fun hideBars() {
        if (sdk >= 30) {
            val c = window.insetsController ?: return
            c.systemBarsBehavior =
                WindowInsetsController.BEHAVIOR_SHOW_TRANSIENT_BARS_BY_SWIPE
            c.hide(WindowInsets.Type.systemBars())
        } else {
            @Suppress("DEPRECATION")
            window.decorView.systemUiVisibility = LEGACY_HIDE
        }
    }

    private fun restoreBars() {
        if (sdk >= 30) {
            val c = window.insetsController ?: return
            c.systemBarsBehavior = savedBehavior
            if (savedStatusVisible) c.show(WindowInsets.Type.statusBars())
            if (savedNavVisible) c.show(WindowInsets.Type.navigationBars())
        } else {
            @Suppress("DEPRECATION")
            window.decorView.systemUiVisibility = savedUiVisibility
        }
    }

    private fun report() {
        val fs = want && !activity.isInMultiWindowMode
        glView.queueEvent { Androidapp.fullscreenChanged(fs) }
    }

    private companion object {
        @Suppress("DEPRECATION")
        val LEGACY_HIDE = View.SYSTEM_UI_FLAG_IMMERSIVE_STICKY or
            View.SYSTEM_UI_FLAG_FULLSCREEN or
            View.SYSTEM_UI_FLAG_HIDE_NAVIGATION or
            View.SYSTEM_UI_FLAG_LAYOUT_STABLE or
            View.SYSTEM_UI_FLAG_LAYOUT_FULLSCREEN or
            View.SYSTEM_UI_FLAG_LAYOUT_HIDE_NAVIGATION
    }
}
