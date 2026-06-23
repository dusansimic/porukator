package org.porukator.app.net

import com.connectrpc.ProtocolClientConfig
import com.connectrpc.extensions.GoogleJavaLiteProtobufStrategy
import com.connectrpc.impl.ProtocolClient
import com.connectrpc.okhttp.ConnectOkHttpClient
import com.connectrpc.protocols.NetworkProtocol
import okhttp3.OkHttpClient
import org.porukator.app.data.ConnectionConfig
import porukator.v1.ClientServiceClient
import java.util.concurrent.TimeUnit

// Builds a ClientService client bound to a host. Connection auth is supplied
// per-call via the Authorization header (see authHeaders), since the device
// identity is derived from its access token server-side.
object Clients {
    fun clientService(config: ConnectionConfig): ClientServiceClient {
        val okhttp = OkHttpClient.Builder()
            // Server streams stay open indefinitely; disable the read timeout.
            .readTimeout(0, TimeUnit.MILLISECONDS)
            .pingInterval(30, TimeUnit.SECONDS)
            .build()

        val protocolClient = ProtocolClient(
            httpClient = ConnectOkHttpClient(okhttp),
            ProtocolClientConfig(
                host = config.host.trimEnd('/'),
                serializationStrategy = GoogleJavaLiteProtobufStrategy(),
                networkProtocol = NetworkProtocol.CONNECT,
            ),
        )
        return ClientServiceClient(protocolClient)
    }

    fun authHeaders(token: String): Map<String, List<String>> =
        mapOf("Authorization" to listOf("Bearer $token"))
}
