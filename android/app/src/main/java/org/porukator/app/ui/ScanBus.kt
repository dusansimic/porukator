package org.porukator.app.ui

import kotlinx.coroutines.flow.MutableStateFlow

// Carries a scanned QR payload from the scanner screen back to the setup screen.
object ScanBus {
    val lastScan = MutableStateFlow<String?>(null)
    fun consume(): String? = lastScan.getAndSet(null)
}

private fun <T> MutableStateFlow<T>.getAndSet(value: T): T {
    val prev = this.value
    this.value = value
    return prev
}
