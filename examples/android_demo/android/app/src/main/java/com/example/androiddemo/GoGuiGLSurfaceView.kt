package com.example.androiddemo

import android.app.Activity
import android.content.Context
import android.opengl.GLSurfaceView
import android.text.InputType
import android.view.KeyEvent
import android.view.MotionEvent
import android.view.accessibility.AccessibilityNodeProvider
import android.view.inputmethod.BaseInputConnection
import android.view.inputmethod.EditorInfo
import android.view.inputmethod.InputConnection
import android.os.Build
import android.view.WindowInsets
import androidapp.Androidapp

class GoGuiGLSurfaceView(context: Context) : GLSurfaceView(context) {

    private val density = resources.displayMetrics.density
    private val a11yProvider = GoGuiAccessibilityProvider(this, density)

    init {
        setEGLContextClientVersion(3)
        // 8-bit stencil required for ClipContents.
        setEGLConfigChooser(8, 8, 8, 8, 0, 8)
        // Keep the GL context while the app is in the background, so a
        // resume does not have to rebuild every texture and shader.
        // Android may still drop it; the backend then rebuilds them
        // when GoGuiRenderer.onSurfaceCreated reports the new context.
        preserveEGLContextOnPause = true
        setRenderer(GoGuiRenderer(density, context as Activity, this))
        renderMode = RENDERMODE_CONTINUOUSLY
        isFocusable = true
        isFocusableInTouchMode = true
        importantForAccessibility = IMPORTANT_FOR_ACCESSIBILITY_YES
        // Report the safe area (issue #1171) and how much of the view
        // the soft keyboard covers (issue #770). The activity uses
        // adjustNothing, so the surface keeps its size and the app
        // decides what to move. Insets change on rotation and when bars
        // show or hide; the view's place in the window changes on
        // layout, so both report.
        setOnApplyWindowInsetsListener { _, insets ->
            reportInsets()
            insets
        }
        addOnLayoutChangeListener { _, _, _, _, _, _, _, _, _ ->
            reportInsets()
        }
    }

    private var lastSafeArea = FloatArray(4) { -1f }
    private var lastKeyboard = -1f

    /** Sends the safe area and the keyboard height to Go, in logical
     *  pixels, measured against where this view sits in the window.
     *  The window's insets are relative to the window's edges, and
     *  the view may not reach them: below Android 15 it can sit
     *  between the system bars, from Android 15 it reaches under
     *  them. An inset counts only where it overlaps the view, so the
     *  navigation bar is not subtracted from the keyboard by guess.
     *  IME insets need API 30; below that the keyboard height stays
     *  0, which gui documents as "cannot report". */
    private fun reportInsets() {
        val insets = rootWindowInsets ?: return
        val root = rootView
        val loc = IntArray(2)
        getLocationInWindow(loc)
        val gapLeft = loc[0]
        val gapTop = loc[1]
        val gapRight = root.width - (loc[0] + width)
        val gapBottom = root.height - (loc[1] + height)

        var left: Int
        var top: Int
        var right: Int
        var bottom: Int
        var ime = 0
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.R) {
            // Hidden bars report 0 here; the cutout stays.
            val s = insets.getInsets(
                WindowInsets.Type.systemBars() or
                    WindowInsets.Type.displayCutout()
            )
            left = s.left
            top = s.top
            right = s.right
            bottom = s.bottom
            ime = insets.getInsets(WindowInsets.Type.ime()).bottom
        } else {
            @Suppress("DEPRECATION")
            left = insets.systemWindowInsetLeft
            @Suppress("DEPRECATION")
            top = insets.systemWindowInsetTop
            @Suppress("DEPRECATION")
            right = insets.systemWindowInsetRight
            @Suppress("DEPRECATION")
            bottom = insets.systemWindowInsetBottom
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.P) {
                insets.displayCutout?.let {
                    left = maxOf(left, it.safeInsetLeft)
                    top = maxOf(top, it.safeInsetTop)
                    right = maxOf(right, it.safeInsetRight)
                    bottom = maxOf(bottom, it.safeInsetBottom)
                }
            }
        }

        val safe = floatArrayOf(
            maxOf(0, top - gapTop) / density,
            maxOf(0, right - gapRight) / density,
            maxOf(0, bottom - gapBottom) / density,
            maxOf(0, left - gapLeft) / density,
        )
        val keyboard = maxOf(0, ime - gapBottom) / density
        if (!safe.contentEquals(lastSafeArea)) {
            lastSafeArea = safe
            queueEvent {
                Androidapp.safeAreaInsets(safe[0], safe[1], safe[2], safe[3])
            }
        }
        if (keyboard != lastKeyboard) {
            lastKeyboard = keyboard
            queueEvent { Androidapp.softKeyboardInset(keyboard) }
        }
    }

    /** Maps the focused field's gui.KeyboardKind (issue #770) to an
     *  InputType. Kind values: 0=text, 1=number, 2=decimal,
     *  3=phone, 4=email, 5=url. */
    private fun inputTypeFor(kind: Int, secure: Boolean): Int {
        val numeric = kind == 1 || kind == 2
        if (secure) {
            return if (numeric) {
                InputType.TYPE_CLASS_NUMBER or
                    InputType.TYPE_NUMBER_VARIATION_PASSWORD
            } else {
                InputType.TYPE_CLASS_TEXT or
                    InputType.TYPE_TEXT_VARIATION_PASSWORD
            }
        }
        return when (kind) {
            1 -> InputType.TYPE_CLASS_NUMBER or
                InputType.TYPE_NUMBER_FLAG_SIGNED
            2 -> InputType.TYPE_CLASS_NUMBER or
                InputType.TYPE_NUMBER_FLAG_SIGNED or
                InputType.TYPE_NUMBER_FLAG_DECIMAL
            3 -> InputType.TYPE_CLASS_PHONE
            4 -> InputType.TYPE_CLASS_TEXT or
                InputType.TYPE_TEXT_VARIATION_EMAIL_ADDRESS
            5 -> InputType.TYPE_CLASS_TEXT or
                InputType.TYPE_TEXT_VARIATION_URI
            else -> InputType.TYPE_CLASS_TEXT
        }
    }

    // --- Accessibility ---

    override fun getAccessibilityNodeProvider(): AccessibilityNodeProvider {
        return a11yProvider
    }

    /** Called from the renderer's onDrawFrame to check for
     *  pending accessibility announcements. */
    fun pollA11yAnnouncement() {
        val text = Androidapp.pendingA11yAnnouncement()
        if (text.isNotEmpty()) {
            post { announceForAccessibility(text) }
        }
    }

    override fun onTouchEvent(event: MotionEvent): Boolean {
        val action = event.actionMasked
        val pointerIndex = event.actionIndex

        when (action) {
            MotionEvent.ACTION_DOWN,
            MotionEvent.ACTION_POINTER_DOWN -> {
                val id = event.getPointerId(pointerIndex).toLong()
                val x = event.getX(pointerIndex) / density
                val y = event.getY(pointerIndex) / density
                queueEvent { Androidapp.touchInput(0, id, x, y) }
            }
            MotionEvent.ACTION_MOVE -> {
                for (i in 0 until event.pointerCount) {
                    val id = event.getPointerId(i).toLong()
                    val x = event.getX(i) / density
                    val y = event.getY(i) / density
                    queueEvent { Androidapp.touchInput(1, id, x, y) }
                }
            }
            MotionEvent.ACTION_UP,
            MotionEvent.ACTION_POINTER_UP -> {
                val id = event.getPointerId(pointerIndex).toLong()
                val x = event.getX(pointerIndex) / density
                val y = event.getY(pointerIndex) / density
                queueEvent { Androidapp.touchInput(2, id, x, y) }
            }
            MotionEvent.ACTION_CANCEL -> {
                for (i in 0 until event.pointerCount) {
                    val id = event.getPointerId(i).toLong()
                    val x = event.getX(i) / density
                    val y = event.getY(i) / density
                    queueEvent { Androidapp.touchInput(3, id, x, y) }
                }
            }
        }
        return true
    }

    // --- IME support ---

    override fun onCheckIsTextEditor(): Boolean = true

    override fun onCreateInputConnection(
        outAttrs: EditorInfo
    ): InputConnection {
        outAttrs.inputType = inputTypeFor(
            Androidapp.pendingIMEKeyboardKind().toInt(),
            Androidapp.pendingIMEKeyboardSecure()
        )
        outAttrs.imeOptions = EditorInfo.IME_FLAG_NO_FULLSCREEN or
            EditorInfo.IME_FLAG_NO_EXTRACT_UI
        return object : BaseInputConnection(this, false) {
            // The view sees none of the field's text, so the keyboard
            // sends Backspace and Enter as key events instead of
            // editing. They travel as commits Go maps to key-downs.
            override fun sendKeyEvent(event: KeyEvent): Boolean {
                val text = when (event.keyCode) {
                    KeyEvent.KEYCODE_DEL -> "\b"
                    KeyEvent.KEYCODE_FORWARD_DEL -> "\u007F"
                    KeyEvent.KEYCODE_ENTER,
                    KeyEvent.KEYCODE_NUMPAD_ENTER -> "\n"
                    else -> return super.sendKeyEvent(event)
                }
                if (event.action == KeyEvent.ACTION_DOWN) {
                    queueEvent { Androidapp.imeCommit(text) }
                }
                return true
            }

            override fun commitText(
                text: CharSequence,
                newCursorPosition: Int
            ): Boolean {
                queueEvent { Androidapp.imeCommit(text.toString()) }
                return true
            }

            override fun setComposingText(
                text: CharSequence,
                newCursorPosition: Int
            ): Boolean {
                queueEvent {
                    Androidapp.imeComposition(
                        text.toString(),
                        newCursorPosition.toLong(),
                        text.length.toLong()
                    )
                }
                return true
            }

            override fun finishComposingText(): Boolean {
                queueEvent {
                    Androidapp.imeComposition("", 0, 0)
                }
                return true
            }

            // The connection has no copy of the field text, so Go applies
            // the requested range at the real field selection. Passing the
            // unit is essential: one emoji is two UTF-16 code units but one
            // Unicode code point.
            private fun deleteSurrounding(
                beforeLength: Int,
                afterLength: Int,
                inCodePoints: Boolean
            ): Boolean {
                if (beforeLength > 0 || afterLength > 0) {
                    queueEvent {
                        Androidapp.imeDeleteSurrounding(
                            beforeLength.toLong(),
                            afterLength.toLong(),
                            inCodePoints
                        )
                    }
                }
                return true
            }

            override fun deleteSurroundingText(
                beforeLength: Int,
                afterLength: Int
            ): Boolean = deleteSurrounding(
                beforeLength, afterLength, false
            )

            // API 24+ keyboards may express the same edit in Unicode code
            // points. The view still owns no editable buffer, so the base
            // implementation cannot apply it to the go-gui field.
            override fun deleteSurroundingTextInCodePoints(
                beforeLength: Int,
                afterLength: Int
            ): Boolean = deleteSurrounding(
                beforeLength, afterLength, true
            )
        }
    }
}
