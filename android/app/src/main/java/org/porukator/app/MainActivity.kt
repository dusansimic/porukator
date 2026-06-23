package org.porukator.app

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.darkColorScheme
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.rememberNavController
import org.porukator.app.ui.QrScannerScreen
import org.porukator.app.ui.SetupScreen
import org.porukator.app.ui.StatusScreen

class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContent {
            MaterialTheme(colorScheme = darkColorScheme()) {
                Surface {
                    val nav = rememberNavController()
                    NavHost(navController = nav, startDestination = "status") {
                        composable("status") {
                            StatusScreen(onEditConnection = { nav.navigate("setup") })
                        }
                        composable("setup") {
                            SetupScreen(
                                onScan = { nav.navigate("scan") },
                                onSaved = { nav.popBackStack("status", inclusive = false) },
                                onBack = { nav.popBackStack() },
                            )
                        }
                        composable("scan") {
                            QrScannerScreen(
                                onResult = { nav.popBackStack() },
                                onBack = { nav.popBackStack() },
                            )
                        }
                    }
                }
            }
        }
    }
}
