package dev.mygo

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.Context
import android.content.Intent
import android.net.ConnectivityManager
import android.net.Network
import android.net.NetworkCapabilities
import android.net.NetworkRequest
import android.net.VpnService
import android.os.Build
import android.os.ParcelFileDescriptor
import org.json.JSONObject

class NetLinkService : VpnService() {
    companion object {
        const val ACTION_START = "io.github.mumulhl.looma.START"
        const val ACTION_STOP = "io.github.mumulhl.looma.STOP"
        const val EXTRA_CONFIG_JSON = "config_json"
        const val EXTRA_PER_APP_MODE = "per_app_mode"
        const val EXTRA_PER_APP_PACKAGES = "per_app_packages"
        const val EXTRA_ALLOW_LAN = "allow_lan"
        const val EXTRA_KILL_SWITCH = "kill_switch"
        const val EXTRA_CHAIN_DESC = "chain_desc"
        private const val CHANNEL_ID = "looma_status_channel"
        private const val NOTIF_ID = 1001

        @Volatile
        private var instance: NetLinkService? = null

        fun updateNotificationStats(chain: String, upBps: Long, downBps: Long) {
            instance?.refreshNotification(chain, upBps, downBps)
        }
    }

    private var tunInterface: ParcelFileDescriptor? = null
    private var networkCallback: ConnectivityManager.NetworkCallback? = null
    private var currentChain: String = "Looma"

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        when (intent?.action) {
            ACTION_STOP -> {
                stopServiceSession()
                return START_NOT_STICKY
            }
            ACTION_START -> {
                instance = this
                currentChain = intent.getStringExtra(EXTRA_CHAIN_DESC) ?: "Looma"
                val perAppMode = intent.getStringExtra(EXTRA_PER_APP_MODE) ?: "all"
                val pkgsRaw = intent.getStringExtra(EXTRA_PER_APP_PACKAGES) ?: ""
                val packages = pkgsRaw.split(",").map { it.trim() }.filter { it.isNotEmpty() }
                val allowLan = intent.getBooleanExtra(EXTRA_ALLOW_LAN, false)
                val killSwitch = intent.getBooleanExtra(EXTRA_KILL_SWITCH, false)

                ensureNotificationChannel()
                startForeground(NOTIF_ID, buildNotification(currentChain, 0, 0))
                establishTun(perAppMode, packages, allowLan, killSwitch)
                registerNetworkMonitor()
            }
        }
        return START_STICKY
    }

    private fun establishTun(perAppMode: String, packages: List<String>, allowLan: Boolean, killSwitch: Boolean) {
        tunInterface?.close()
        val builder = Builder()
            .setSession("Looma")
            .setMtu(1500)
            .addAddress("172.19.0.1", 30)
            .addDnsServer("172.19.0.2")
            .addRoute("0.0.0.0", 0)
            .setBlocking(killSwitch)

        if (killSwitch) {
            // Capture IPv6 inside TUN so dropped tunnels cannot leak via direct IPv6.
            builder.addAddress("fdfe:dcba:9876::1", 126)
            builder.addRoute("::", 0)
        }

        // VpnService.Builder forbids mixing addAllowedApplication and addDisallowedApplication.
        when (perAppMode) {
            "whitelist" -> {
                for (pkg in packages) {
                    if (pkg != packageName) {
                        try { builder.addAllowedApplication(pkg) } catch (_: Exception) {}
                    }
                }
            }
            "blacklist" -> {
                try { builder.addDisallowedApplication(packageName) } catch (_: Exception) {}
                for (pkg in packages) {
                    try { builder.addDisallowedApplication(pkg) } catch (_: Exception) {}
                }
            }
            else -> {
                try { builder.addDisallowedApplication(packageName) } catch (_: Exception) {}
            }
        }

        if (allowLan && Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
            builder.excludeRoute(android.net.IpPrefix(java.net.InetAddress.getByName("192.168.0.0"), 16))
            builder.excludeRoute(android.net.IpPrefix(java.net.InetAddress.getByName("10.0.0.0"), 8))
            builder.excludeRoute(android.net.IpPrefix(java.net.InetAddress.getByName("172.16.0.0"), 12))
        }

        tunInterface = builder.establish()
        val fd = tunInterface?.fd ?: -1
        PluginHost.emit("android_netlink", "tun_ready", JSONObject().put("fd", fd))
    }

    private fun registerNetworkMonitor() {
        val cm = getSystemService(Context.CONNECTIVITY_SERVICE) as ConnectivityManager
        networkCallback?.let { try { cm.unregisterNetworkCallback(it) } catch (_: Exception) {} }
        val req = NetworkRequest.Builder()
            .addCapability(NetworkCapabilities.NET_CAPABILITY_INTERNET)
            .addCapability(NetworkCapabilities.NET_CAPABILITY_NOT_VPN)
            .build()
        val cb = object : ConnectivityManager.NetworkCallback() {
            override fun onAvailable(network: Network) {
                PluginHost.emit("android_netlink", "network_changed", JSONObject().put("state", "available"))
            }
            override fun onLost(network: Network) {
                PluginHost.emit("android_netlink", "network_changed", JSONObject().put("state", "lost"))
            }
        }
        networkCallback = cb
        cm.registerNetworkCallback(req, cb)
    }

    private fun stopServiceSession() {
        val cm = getSystemService(Context.CONNECTIVITY_SERVICE) as ConnectivityManager
        networkCallback?.let { try { cm.unregisterNetworkCallback(it) } catch (_: Exception) {} }
        networkCallback = null
        tunInterface?.close()
        tunInterface = null
        PluginHost.emit("android_netlink", "tun_stopped", JSONObject())
        stopForeground(STOP_FOREGROUND_REMOVE)
        stopSelf()
        instance = null
    }

    override fun onDestroy() {
        stopServiceSession()
        super.onDestroy()
    }

    private fun ensureNotificationChannel() {
        val nm = getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager
        val ch = NotificationChannel(CHANNEL_ID, "Looma Status", NotificationManager.IMPORTANCE_LOW)
        nm.createNotificationChannel(ch)
    }

    private fun refreshNotification(chain: String, upBps: Long, downBps: Long) {
        val nm = getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager
        nm.notify(NOTIF_ID, buildNotification(chain, upBps, downBps))
    }

    private fun buildNotification(chain: String, upBps: Long, downBps: Long): Notification {
        val stopIntent = Intent(this, NetLinkService::class.java).apply { action = ACTION_STOP }
        val stopPi = PendingIntent.getService(this, 0, stopIntent, PendingIntent.FLAG_IMMUTABLE)
        val text = "↑ ${formatRate(upBps)}  ↓ ${formatRate(downBps)}"
        return Notification.Builder(this, CHANNEL_ID)
            .setContentTitle("Looma · $chain")
            .setContentText(text)
            .setOngoing(true)
            .setSmallIcon(android.R.drawable.ic_lock_lock)
            .addAction(Notification.Action.Builder(null, "Disconnect", stopPi).build())
            .build()
    }

    private fun formatRate(bps: Long): String = when {
        bps >= 1024 * 1024 -> String.format("%.1f MB/s", bps / (1024.0 * 1024.0))
        bps >= 1024 -> String.format("%.1f KB/s", bps / 1024.0)
        else -> "$bps B/s"
    }
}
