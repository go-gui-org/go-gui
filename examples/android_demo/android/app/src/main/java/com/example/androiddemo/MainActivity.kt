package com.example.androiddemo

import android.app.Activity
import android.app.NotificationChannel
import android.app.NotificationManager
import android.content.res.Configuration
import android.os.Build
import android.os.Bundle
import androidx.core.app.NotificationCompat
import androidx.core.app.NotificationManagerCompat
import androidapp.Androidapp
import java.util.concurrent.atomic.AtomicInteger

class MainActivity : Activity() {

    private lateinit var glSurfaceView: GoGuiGLSurfaceView

    companion object {
        private const val CHANNEL_ID = "go_gui_default"
        private val notifId = AtomicInteger(0)
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        createNotificationChannel()
        Androidapp.init()
        glSurfaceView = GoGuiGLSurfaceView(this)
        setContentView(glSurfaceView)
        pushAppearance(resources.configuration)
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

    override fun onResume() {
        super.onResume()
        glSurfaceView.onResume()
    }

    override fun onPause() {
        glSurfaceView.onPause()
        super.onPause()
    }

    override fun onDestroy() {
        glSurfaceView.queueEvent {
            Androidapp.cleanUp()
        }
        super.onDestroy()
    }

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
            val builder = NotificationCompat.Builder(this, CHANNEL_ID)
                .setSmallIcon(android.R.drawable.ic_dialog_info)
                .setContentTitle(title)
                .setContentText(body)
                .setPriority(NotificationCompat.PRIORITY_DEFAULT)

            val manager = NotificationManagerCompat.from(this)
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
