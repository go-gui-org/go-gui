package com.example.androiddemo

import android.app.Activity
import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.content.res.Configuration
import android.os.Build
import android.os.Bundle
import androidapp.Androidapp
import java.util.concurrent.atomic.AtomicInteger

class MainActivity : Activity() {

    private lateinit var glSurfaceView: GoGuiGLSurfaceView
    // Window.SetFullscreen (go-gui #1171).
    private lateinit var fullscreen: GoGuiFullscreen

    companion object {
        private const val CHANNEL_ID = "go_gui_default"
        private val notifId = AtomicInteger(0)
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        createNotificationChannel()
        Androidapp.init()
        // Where gui.SaveSettings keeps the app's settings (go-gui #848).
        Androidapp.setFilesDir(filesDir.absolutePath)
        Androidapp.setCacheDir(cacheDir.absolutePath)
        Androidapp.setSecretStore(AndroidSecretStore(applicationContext))
        glSurfaceView = GoGuiGLSurfaceView(this)
        setContentView(glSurfaceView)
        fullscreen = GoGuiFullscreen(this, glSurfaceView)
        pushAppearance(resources.configuration)
    }

    // setFullscreen applies a request GoGuiRenderer polled. UI thread.
    fun setFullscreen(on: Boolean) = fullscreen.set(on)

    override fun onWindowFocusChanged(hasFocus: Boolean) {
        super.onWindowFocusChanged(hasFocus)
        fullscreen.onWindowFocusChanged(hasFocus)
    }

    // The one-argument form reaches Android 7; Android 8 and later call
    // it from the two-argument one.
    @Suppress("OVERRIDE_DEPRECATION", "DEPRECATION")
    override fun onMultiWindowModeChanged(isInMultiWindowMode: Boolean) {
        super.onMultiWindowModeChanged(isInMultiWindowMode)
        fullscreen.onMultiWindowModeChanged()
    }

    override fun onConfigurationChanged(newConfig: Configuration) {
        super.onConfigurationChanged(newConfig)
        pushAppearance(newConfig)
    }

    // pushAppearance forwards the uiMode night flag to Go (issue
    // #752). Delivered on the GL thread like every other bridge
    // call. Needs uiMode in configChanges, or the activity
    // restarts on toggle instead of landing here.
    private fun pushAppearance(config: Configuration) {
        val dark = config.uiMode and Configuration.UI_MODE_NIGHT_MASK ==
            Configuration.UI_MODE_NIGHT_YES
        glSurfaceView.queueEvent { Androidapp.appearanceChanged(dark) }
    }

    // onStart / onStop tell Go whether the app is on screen (go-gui
    // #1170). Not onResume / onPause: in multi-window mode a paused
    // activity is still visible. The GL thread runs a queued event
    // while it is paused, so the report from onStop arrives at once.
    //
    // A stop for a configuration change is not reported: Android
    // replaces the activity within milliseconds (leaving split screen
    // does this), so the app never leaves the screen. Reporting it
    // would also race: the old activity's "hidden" runs on the old GL
    // thread and the new activity's "shown" on the new one, and a
    // "hidden" that came last would leave the app marked hidden.
    override fun onStart() {
        super.onStart()
        glSurfaceView.queueEvent { Androidapp.visibilityChanged(true) }
    }

    override fun onStop() {
        if (!isChangingConfigurations) {
            glSurfaceView.queueEvent { Androidapp.visibilityChanged(false) }
        }
        super.onStop()
    }

    override fun onResume() {
        super.onResume()
        glSurfaceView.onResume()
    }

    override fun onPause() {
        glSurfaceView.onPause()
        super.onPause()
    }

    // The host does nothing when the activity is destroyed. The Go window
    // and backend live as long as the app's process. A new activity reuses
    // them, with the app's state, and onSurfaceCreated rebuilds the GL
    // resources (go-gui #1246).

    private fun createNotificationChannel() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            val channel = NotificationChannel(
                CHANNEL_ID,
                "Default",
                NotificationManager.IMPORTANCE_DEFAULT
            )
            val manager = getSystemService(
                NotificationManager::class.java
            )
            manager.createNotificationChannel(channel)
        }
    }

    fun postNotification(title: String, body: String) {
        try {
            val builder = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
                Notification.Builder(this, CHANNEL_ID)
            } else {
                @Suppress("DEPRECATION")
                Notification.Builder(this)
            }
            builder
                .setSmallIcon(android.R.drawable.ic_dialog_info)
                .setContentTitle(title)
                .setContentText(body)
                .setPriority(Notification.PRIORITY_DEFAULT)

            val manager = getSystemService(NotificationManager::class.java)
            manager.notify(notifId.getAndIncrement(), builder.build())

            glSurfaceView.queueEvent {
                Androidapp.notificationResult(0, "", "")
            }
        } catch (e: SecurityException) {
            glSurfaceView.queueEvent {
                Androidapp.notificationResult(
                    1, "permission_denied", e.message ?: ""
                )
            }
        } catch (e: Exception) {
            glSurfaceView.queueEvent {
                Androidapp.notificationResult(
                    2, "error", e.message ?: ""
                )
            }
        }
    }
}
