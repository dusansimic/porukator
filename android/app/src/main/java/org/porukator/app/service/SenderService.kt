package org.porukator.app.service

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.Service
import android.content.Context
import android.content.Intent
import android.os.Build
import android.os.IBinder
import android.telephony.SmsManager
import android.util.Log
import com.google.protobuf.Timestamp
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Job
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch
import org.porukator.app.data.ConnectionStore
import org.porukator.app.net.Clients
import porukator.v1.Porukator
import kotlin.random.Random

// Foreground service that holds the job stream open, sends each pushed SMS with
// the configured delay + jitter, and reports every outcome back to the service.
class SenderService : Service() {

    private val scope = CoroutineScope(SupervisorJob())
    private var worker: Job? = null

    override fun onBind(intent: Intent?): IBinder? = null

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        startForeground(NOTIF_ID, buildNotification("Connecting…"))
        SenderState.setRunning(true)
        if (worker == null) {
            worker = scope.launch { runLoop() }
        }
        return START_STICKY
    }

    override fun onDestroy() {
        SenderState.setRunning(false)
        SenderState.setConnected(false)
        scope.cancel()
        super.onDestroy()
    }

    // Connect, stream jobs, and send. Reconnect with capped backoff on any error
    // or clean stream close, until the service is stopped.
    private suspend fun runLoop() {
        var backoffMs = 1000L
        while (scope.isActive) {
            val config = ConnectionStore.flow(applicationContext).first()
            if (!config.isComplete) {
                SenderState.setError("No connection configured")
                delay(5000)
                continue
            }
            try {
                val svc = Clients.clientService(config)
                val stream = svc.streamJobs(Clients.authHeaders(config.token))
                stream.sendAndClose(Porukator.StreamJobsRequest.getDefaultInstance())
                SenderState.setConnected(true)
                SenderState.setError("")
                updateNotification("Online — waiting for messages")
                backoffMs = 1000L

                for (job in stream.responseChannel()) {
                    sendAndReport(svc, config.token, job)
                    val jitter = if (job.jitterMs > 0) Random.nextInt(job.jitterMs) else 0
                    delay((job.delayMs + jitter).toLong().coerceAtLeast(0))
                }
            } catch (t: Throwable) {
                Log.w(TAG, "stream error", t)
                SenderState.setError(t.message ?: "stream error")
            }

            SenderState.setConnected(false)
            updateNotification("Disconnected — retrying…")
            if (!scope.isActive) break
            delay(backoffMs)
            backoffMs = (backoffMs * 2).coerceAtMost(30000L)
        }
    }

    private suspend fun sendAndReport(
        svc: porukator.v1.ClientServiceClient,
        token: String,
        job: Porukator.Job,
    ) {
        var ok = true
        var errMsg = ""
        try {
            val sms = smsManager()
            val parts = sms.divideMessage(job.content)
            sms.sendMultipartTextMessage(job.phoneNumber, null, parts, null, null)
        } catch (t: Throwable) {
            ok = false
            errMsg = t.message ?: "send failed"
            Log.w(TAG, "sms send failed", t)
        }

        val now = System.currentTimeMillis()
        SenderState.recordSend(job.phoneNumber, ok, errMsg, now)
        updateNotification("Sent ${SenderState.status.value.sentCount} · failed ${SenderState.status.value.failedCount}")

        val report = Porukator.ReportDeliveryRequest.newBuilder()
            .setMessageId(job.messageId)
            .setSuccess(ok)
            .setSentAt(timestamp(now))
            .setError(errMsg)
            .build()
        runCatching { svc.reportDelivery(report, Clients.authHeaders(token)) }
            .onFailure { Log.w(TAG, "report failed", it) }
    }

    @Suppress("DEPRECATION")
    private fun smsManager(): SmsManager =
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.S) {
            getSystemService(SmsManager::class.java)
        } else {
            SmsManager.getDefault()
        }

    private fun timestamp(epochMs: Long): Timestamp =
        Timestamp.newBuilder()
            .setSeconds(epochMs / 1000)
            .setNanos(((epochMs % 1000) * 1_000_000).toInt())
            .build()

    private fun buildNotification(text: String): Notification {
        val mgr = getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            mgr.createNotificationChannel(
                NotificationChannel(CHANNEL, "Porukator sender", NotificationManager.IMPORTANCE_LOW),
            )
        }
        return Notification.Builder(this, CHANNEL)
            .setContentTitle("Porukator")
            .setContentText(text)
            .setSmallIcon(android.R.drawable.stat_sys_upload)
            .setOngoing(true)
            .build()
    }

    private fun updateNotification(text: String) {
        val mgr = getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager
        mgr.notify(NOTIF_ID, buildNotification(text))
    }

    companion object {
        private const val TAG = "SenderService"
        private const val CHANNEL = "porukator_sender"
        private const val NOTIF_ID = 1

        fun start(context: Context) {
            val intent = Intent(context, SenderService::class.java)
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
                context.startForegroundService(intent)
            } else {
                context.startService(intent)
            }
        }

        fun stop(context: Context) {
            context.stopService(Intent(context, SenderService::class.java))
        }
    }
}
