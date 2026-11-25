package dev.mygo

import android.app.Activity
import android.content.Intent
import android.content.pm.ApplicationInfo
import android.net.VpnService
import org.json.JSONArray
import org.json.JSONObject

class AndroidVpnPlugin : MyGoPlugin {
    override fun call(method: String, args: JSONObject, reply: PluginReply) {
        when (method) {
            "prepareAndStart" -> {
                val activity = PluginHost.activity
                val prepareIntent = VpnService.prepare(activity)
                val startService = {
                    val intent = Intent(activity, MyVpnService::class.java).apply {
                        action = MyVpnService.ACTION_START
                        putExtra(MyVpnService.EXTRA_CONFIG_JSON, args.optString("config_json", ""))
                        putExtra(MyVpnService.EXTRA_PER_APP_MODE, args.optString("per_app_mode", "all"))
                        putExtra(MyVpnService.EXTRA_PER_APP_PACKAGES, args.optString("per_app_packages", ""))
                        putExtra(MyVpnService.EXTRA_ALLOW_LAN, args.optBoolean("allow_lan", false))
                        putExtra(MyVpnService.EXTRA_CHAIN_DESC, args.optString("chain_desc", "MyVPN"))
                    }
                    activity.startForegroundService(intent)
                    reply.success(JSONObject().put("started", true))
                }
                if (prepareIntent != null) {
                    PluginHost.startForResult(prepareIntent) { resultCode, _ ->
                        if (resultCode == Activity.RESULT_OK) {
                            startService()
                        } else {
                            reply.failure(PluginFailure("VpnDenied", "User denied VPN permission"))
                        }
                    }
                } else {
                    startService()
                }
            }

            "stop" -> {
                val activity = PluginHost.activity
                val intent = Intent(activity, MyVpnService::class.java).apply {
                    action = MyVpnService.ACTION_STOP
                }
                activity.startService(intent)
                reply.success(JSONObject().put("stopped", true))
            }

            "updateStats" -> {
                val up = args.optLong("up_bps", 0L)
                val down = args.optLong("down_bps", 0L)
                val chain = args.optString("chain_desc", "Connected")
                MyVpnService.updateNotificationStats(chain, up, down)
                reply.success(null)
            }

            "listInstalledApps" -> {
                val pm = PluginHost.activity.packageManager
                val apps = pm.getInstalledApplications(0)
                val arr = JSONArray()
                for (app in apps) {
                    val isSystem = (app.flags and ApplicationInfo.FLAG_SYSTEM) != 0
                    arr.put(
                        JSONObject()
                            .put("package_name", app.packageName)
                            .put("label", pm.getApplicationLabel(app).toString())
                            .put("is_system", isSystem)
                    )
                }
                reply.success(JSONObject().put("apps", arr))
            }

            else -> reply.failure(PluginFailure("NoMethod", method))
        }
    }
}
