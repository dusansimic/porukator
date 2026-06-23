package org.porukator.app.service

import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.update

data class SentEntry(val phone: String, val ok: Boolean, val error: String, val at: Long)

data class SenderStatus(
    val running: Boolean = false,
    val connected: Boolean = false,
    val sentCount: Int = 0,
    val failedCount: Int = 0,
    val lastError: String = "",
    val recent: List<SentEntry> = emptyList(),
)

// Shared, observable state between the foreground service and the UI.
object SenderState {
    private val _status = MutableStateFlow(SenderStatus())
    val status: StateFlow<SenderStatus> = _status

    fun setRunning(running: Boolean) = _status.update { it.copy(running = running) }
    fun setConnected(connected: Boolean) = _status.update { it.copy(connected = connected) }
    fun setError(message: String) = _status.update { it.copy(lastError = message) }

    fun recordSend(phone: String, ok: Boolean, error: String, at: Long) = _status.update {
        it.copy(
            sentCount = it.sentCount + if (ok) 1 else 0,
            failedCount = it.failedCount + if (ok) 0 else 1,
            recent = (listOf(SentEntry(phone, ok, error, at)) + it.recent).take(20),
        )
    }

    fun reset() = _status.update { SenderStatus() }
}
