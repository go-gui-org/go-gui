package com.example.androiddemo

import android.app.Activity
import android.content.Context
import android.opengl.GLSurfaceView
import android.text.InputType
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
        setRenderer(GoGuiRenderer(density, context as Activity, this))
        renderMode = RENDERMODE_CONTINUOUSLY
        isFocusable = true
        isFocusableInTouchMode = true
        importantForAccessibility = IMPORTANT_FOR_ACCESSIBILITY_YES
        // Report how much of the view the soft keyboard covers
        // (issue #770). The activity uses adjustNothing, so the
        // surface keeps its size and the app decides what to move.
        // IME insets need API 30; below that the inset stays 0,
        // which gui documents as "cannot report".
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.R) {
            setOnApplyWindowInsetsListener { _, insets ->
                val ime = insets.getInsets(WindowInsets.Type.ime()).bottom
                val nav = insets.getInsets(
                    WindowInsets.Type.navigationBars()
                ).bottom
                val h = maxOf(0, ime - nav) / density
                queueEvent { Androidapp.softKeyboardInset(h) }
                insets
            }
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

            override fun deleteSurroundingText(
                beforeLength: Int,
                afterLength: Int
            ): Boolean {
                // Backspace for characters before cursor.
                for (i in 0 until beforeLength) {
                    queueEvent { Androidapp.imeCommit("\b") }
                }
                // Delete for characters after cursor.
                for (i in 0 until afterLength) {
                    queueEvent { Androidapp.imeCommit("\u007F") }
                }
                return true
            }
        }
    }
}
