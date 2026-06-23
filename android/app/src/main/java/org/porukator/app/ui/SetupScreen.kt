package org.porukator.app.ui

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Button
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.unit.dp
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.launch
import org.json.JSONObject
import org.porukator.app.data.ConnectionConfig
import org.porukator.app.data.ConnectionStore
import org.porukator.app.service.SenderService

@Composable
fun SetupScreen(onScan: () -> Unit, onSaved: () -> Unit, onBack: () -> Unit) {
    val context = LocalContext.current
    val scope = rememberCoroutineScope()

    var host by remember { mutableStateOf("") }
    var token by remember { mutableStateOf("") }
    var name by remember { mutableStateOf("") }

    // Load any saved config once.
    LaunchedEffect(Unit) {
        val cfg = ConnectionStore.flow(context).first()
        if (host.isBlank()) host = cfg.host
        if (token.isBlank()) token = cfg.token
        if (name.isBlank()) name = cfg.name
    }

    // Apply a scanned QR payload ({host, token, name}) if one is pending.
    LaunchedEffect(Unit) {
        ScanBus.lastScan.collect { payload ->
            if (payload != null) {
                runCatching {
                    val obj = JSONObject(payload)
                    host = obj.optString("host", host)
                    token = obj.optString("token", token)
                    name = obj.optString("name", name)
                }
                ScanBus.consume()
            }
        }
    }

    Column(
        modifier = Modifier.fillMaxSize().padding(24.dp),
        verticalArrangement = Arrangement.spacedBy(16.dp),
    ) {
        Text("Connection", style = androidx.compose.material3.MaterialTheme.typography.headlineSmall)
        OutlinedTextField(value = host, onValueChange = { host = it }, label = { Text("Service host") }, modifier = Modifier.fillMaxWidth(), singleLine = true)
        OutlinedTextField(value = token, onValueChange = { token = it }, label = { Text("Access token") }, modifier = Modifier.fillMaxWidth(), singleLine = true)
        OutlinedTextField(value = name, onValueChange = { name = it }, label = { Text("Device name (optional)") }, modifier = Modifier.fillMaxWidth(), singleLine = true)

        OutlinedButton(onClick = onScan, modifier = Modifier.fillMaxWidth()) { Text("Scan QR code") }

        Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
            OutlinedButton(onClick = onBack) { Text("Cancel") }
            Button(
                onClick = {
                    scope.launch {
                        ConnectionStore.save(context, ConnectionConfig(host.trim(), token.trim(), name.trim()))
                        // Restart the sender so it picks up the new params.
                        SenderService.stop(context)
                        SenderService.start(context)
                        onSaved()
                    }
                },
                enabled = host.isNotBlank() && token.isNotBlank(),
                modifier = Modifier.fillMaxWidth(),
            ) { Text("Save & connect") }
        }
    }
}
