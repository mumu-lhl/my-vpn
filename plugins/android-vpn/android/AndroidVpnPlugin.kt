package dev.mygo

import android.app.Activity
import android.content.Intent
import android.content.pm.ApplicationInfo
import android.net.VpnService
import android.os.Build
import android.provider.Settings
import org.json.JSONArray
import org.json.JSONObject

class AndroidNetPlugin : MyGoPlugin {
    override fun call(method: String, args: JSONObject, reply: PluginReply) {
        when (method) {
            "prepareAndStart" -> {
                val activity = PluginHost.activity ?: run {
                    reply.failure(PluginFailure("NoActivity", "No active activity available"))
                    return
                }
                val prepareIntent = VpnService.prepare(activity)
                val startService = {
                    val intent = Intent(activity, NetLinkService::class.java).apply {
                        action = NetLinkService.ACTION_START
                        putExtra(NetLinkService.EXTRA_CONFIG_JSON, args.optString("config_json", ""))
                        putExtra(NetLinkService.EXTRA_PER_APP_MODE, args.optString("per_app_mode", "all"))
                        putExtra(NetLinkService.EXTRA_PER_APP_PACKAGES, args.optString("per_app_packages", ""))
                        putExtra(NetLinkService.EXTRA_ALLOW_LAN, args.optBoolean("allow_lan", false))
                        putExtra(NetLinkService.EXTRA_KILL_SWITCH, args.optBoolean("kill_switch", false))
                        putExtra(NetLinkService.EXTRA_CHAIN_DESC, args.optString("chain_desc", "Looma"))
                    }
                    activity.startForegroundService(intent)
                    reply.success(JSONObject().put("started", true))
                }
                if (prepareIntent != null) {
                    PluginHost.startForResult(prepareIntent) { resultCode, _ ->
                        if (resultCode == Activity.RESULT_OK) {
                            startService()
                        } else {
                            reply.failure(PluginFailure("PermissionDenied", "User denied network permission"))
                        }
                    }
                } else {
                    startService()
                }
            }

            "stop" -> {
                val ctx = PluginHost.context
                val intent = Intent(ctx, NetLinkService::class.java).apply {
                    action = NetLinkService.ACTION_STOP
                }
                ctx.startService(intent)
                reply.success(JSONObject().put("stopped", true))
            }

            "updateStats" -> {
                val up = args.optLong("up_bps", 0L)
                val down = args.optLong("down_bps", 0L)
                val chain = args.optString("chain_desc", "Connected")
                NetLinkService.updateNotificationStats(chain, up, down)
                reply.success(null)
            }

            "listInstalledApps" -> {
                val ctx = PluginHost.context
                val selfPkg = ctx.packageName
                val pm = ctx.packageManager
                // Query PackageManager off the UI thread to keep the WebView responsive on devices with many apps.
                Thread {
                    try {
                        val apps = pm.getInstalledApplications(0)
                        val arr = JSONArray()
                        for (app in apps) {
                            if (app.packageName == selfPkg) continue
                            val isSystem = (app.flags and ApplicationInfo.FLAG_SYSTEM) != 0
                            val installer = try {
                                if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.R) {
                                    pm.getInstallSourceInfo(app.packageName).installingPackageName ?: ""
                                } else {
                                    @Suppress("DEPRECATION")
                                    pm.getInstallerPackageName(app.packageName) ?: ""
                                }
                            } catch (_: Exception) {
                                ""
                            }
                            arr.put(
                                JSONObject()
                                    .put("package_name", app.packageName)
                                    .put("label", pm.getApplicationLabel(app).toString())
                                    .put("is_system", isSystem)
                                    .put("installer_package", installer)
                            )
                        }
                        reply.success(JSONObject().put("apps", arr))
                    } catch (e: Exception) {
                        reply.failure(PluginFailure("ListAppsFailed", e.message ?: "Failed to list installed apps"))
                    }
                }.start()
            }

            "openVpnSettings" -> {
                val intent = Intent(Settings.ACTION_VPN_SETTINGS).apply {
                    addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
                }
                PluginHost.context.startActivity(intent)
                reply.success(JSONObject().put("opened", true))
            }

            else -> reply.failure(PluginFailure("NoMethod", method))
        }
    }
}
