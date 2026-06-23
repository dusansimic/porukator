package org.porukator.app

import android.Manifest
import android.content.Context
import android.content.pm.PackageManager
import android.os.Build
import androidx.core.content.ContextCompat

// Runtime permissions the sender needs before it can run. SEND_SMS is a
// dangerous permission and must be granted at runtime (a manifest entry alone is
// not enough). POST_NOTIFICATIONS only exists on Android 13+, where the
// foreground-service notification requires it.
val SENDER_PERMISSIONS: Array<String> = buildList {
    add(Manifest.permission.SEND_SMS)
    if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
        add(Manifest.permission.POST_NOTIFICATIONS)
    }
}.toTypedArray()

// hasSmsPermission reports whether SEND_SMS is currently granted.
fun hasSmsPermission(context: Context): Boolean =
    ContextCompat.checkSelfPermission(context, Manifest.permission.SEND_SMS) ==
        PackageManager.PERMISSION_GRANTED
