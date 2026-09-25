package com.example.androiddemo

import android.app.Activity
import android.content.ActivityNotFoundException
import android.content.Intent
import android.net.Uri
import android.opengl.GLSurfaceView
import android.os.Handler
import android.os.Looper
import android.util.Log
import android.view.inputmethod.InputMethodManager
import androidapp.Androidapp
import javax.microedition.khronos.egl.EGLConfig
import javax.microedition.khronos.opengles.GL10

class GoGuiRenderer(
    private val density: Float,
    private val activity: Activity,
    private val glView: GoGuiGLSurfaceView
) : GLSurfaceView.Renderer {

    private val mainHandler = Handler(Looper.getMainLooper())

    override fun onSurfaceCreated(gl: GL10?, config: EGLConfig?) {
        // Nothing — initialization happens in onSurfaceChanged.
    }

    override fun onSurfaceChanged(gl: GL10?, width: Int, height: Int) {
        val logicalW = (width / density).toInt()
        val logicalH = (height / density).toInt()
        Androidapp.start(logicalW.toLong(), logicalH.toLong(), density)
    }

    override fun onDrawFrame(gl: GL10?) {
        Androidapp.render()
        pollPendingURI()
        pollPendingIMEAction()
        pollPendingNotification()
        glView.pollA11yAnnouncement()
    }

    private fun pollPendingURI() {
        val uri = Androidapp.pendingURI()
        if (uri.isNotEmpty()) {
            mainHandler.post {
                try {
                    val intent = Intent(Intent.ACTION_VIEW, Uri.parse(uri))
                    activity.startActivity(intent)
                } catch (e: ActivityNotFoundException) {
                    Log.w("GoGui", "no activity for URI: $uri")
                }
            }
        }
    }

    // Keyboard kind of the last show, so a field of another kind
    // restarts the InputConnection (issue #770). Main thread only.
    private var shownKind = -1
    private var shownSecure = false

    private fun pollPendingIMEAction() {
        when (Androidapp.pendingIMEAction().toInt()) {
            1 -> mainHandler.post {
                glView.requestFocus()
                val imm = activity.getSystemService(
                    Activity.INPUT_METHOD_SERVICE
                ) as InputMethodManager
                val kind = Androidapp.pendingIMEKeyboardKind().toInt()
                val secure = Androidapp.pendingIMEKeyboardSecure()
                if (kind != shownKind || secure != shownSecure) {
                    shownKind = kind
                    shownSecure = secure
                    // Re-runs onCreateInputConnection, which reads
                    // the new kind into EditorInfo.inputType.
                    imm.restartInput(glView)
                }
                imm.showSoftInput(glView, InputMethodManager.SHOW_IMPLICIT)
            }
            2 -> mainHandler.post {
                val imm = activity.getSystemService(
                    Activity.INPUT_METHOD_SERVICE
                ) as InputMethodManager
                imm.hideSoftInputFromWindow(glView.windowToken, 0)
            }
        }
    }

    private fun pollPendingNotification() {
        val title = Androidapp.pendingNotificationTitle()
        if (title.isNotEmpty()) {
            val body = Androidapp.pendingNotificationBody()
            mainHandler.post {
                (activity as? MainActivity)?.postNotification(title, body)
            }
        }
    }
}
