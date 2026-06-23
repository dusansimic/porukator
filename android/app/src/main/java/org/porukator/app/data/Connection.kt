package org.porukator.app.data

import android.content.Context
import androidx.datastore.preferences.core.edit
import androidx.datastore.preferences.core.stringPreferencesKey
import androidx.datastore.preferences.preferencesDataStore
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.map

// Connection parameters the device uses to reach the Porukator service. Entered
// manually or scanned from the QR shown in the web UI.
data class ConnectionConfig(
    val host: String,
    val token: String,
    val name: String,
) {
    val isComplete: Boolean get() = host.isNotBlank() && token.isNotBlank()
}

private val Context.dataStore by preferencesDataStore(name = "porukator")

object ConnectionStore {
    private val HOST = stringPreferencesKey("host")
    private val TOKEN = stringPreferencesKey("token")
    private val NAME = stringPreferencesKey("name")

    fun flow(context: Context): Flow<ConnectionConfig> =
        context.dataStore.data.map {
            ConnectionConfig(
                host = it[HOST] ?: "",
                token = it[TOKEN] ?: "",
                name = it[NAME] ?: "",
            )
        }

    suspend fun save(context: Context, config: ConnectionConfig) {
        context.dataStore.edit {
            it[HOST] = config.host
            it[TOKEN] = config.token
            it[NAME] = config.name
        }
    }

    suspend fun clear(context: Context) {
        context.dataStore.edit { it.clear() }
    }
}
