package org.porukator.app.ui

import android.Manifest
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import kotlinx.coroutines.flow.map
import org.porukator.app.SENDER_PERMISSIONS
import org.porukator.app.data.ConnectionStore
import org.porukator.app.service.SenderService
import org.porukator.app.service.SenderState
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale

@Composable
fun StatusScreen(onEditConnection: () -> Unit) {
    val context = LocalContext.current
    val status by SenderState.status.collectAsState()
    val name by remember { ConnectionStore.flow(context).map { it.name } }.collectAsState(initial = "")
    val configured by remember { ConnectionStore.flow(context).map { it.isComplete } }.collectAsState(initial = false)

    // Request the runtime permissions the sender needs, then start it.
    val launcher = rememberLauncherForActivityResult(
        ActivityResultContracts.RequestMultiplePermissions(),
    ) { granted ->
        if (granted[Manifest.permission.SEND_SMS] == true) {
            SenderService.start(context)
        }
    }

    Column(
        modifier = Modifier.fillMaxSize().padding(24.dp),
        verticalArrangement = Arrangement.spacedBy(16.dp),
    ) {
        Text("Porukator", style = MaterialTheme.typography.headlineMedium)
        Text(if (name.isNotBlank()) "Device: $name" else "Unnamed device", style = MaterialTheme.typography.bodyMedium)

        Card(modifier = Modifier.fillMaxWidth()) {
            Column(modifier = Modifier.padding(16.dp), verticalArrangement = Arrangement.spacedBy(4.dp)) {
                val state = when {
                    !status.running -> "Stopped"
                    status.connected -> "Online"
                    else -> "Connecting…"
                }
                Text("Status: $state", style = MaterialTheme.typography.titleMedium)
                Text("Sent: ${status.sentCount}   Failed: ${status.failedCount}")
                if (status.lastError.isNotBlank()) {
                    Text("Last error: ${status.lastError}", color = MaterialTheme.colorScheme.error)
                }
            }
        }

        Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
            if (!status.running) {
                Button(
                    onClick = { launcher.launch(SENDER_PERMISSIONS) },
                    enabled = configured,
                ) { Text("Start") }
            } else {
                OutlinedButton(onClick = { SenderService.stop(context) }) { Text("Stop") }
            }
            OutlinedButton(onClick = onEditConnection) { Text("Connection") }
        }

        Text("Recent", style = MaterialTheme.typography.titleSmall)
        LazyColumn(verticalArrangement = Arrangement.spacedBy(6.dp)) {
            items(status.recent) { entry ->
                val time = SimpleDateFormat("HH:mm:ss", Locale.getDefault()).format(Date(entry.at))
                Text(
                    "$time  ${if (entry.ok) "✓" else "✗"}  ${entry.phone}" +
                        if (entry.error.isNotBlank()) "  ${entry.error}" else "",
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                    style = MaterialTheme.typography.bodySmall,
                )
            }
        }
    }
}
