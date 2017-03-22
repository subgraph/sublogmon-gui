# eventnotifier
Subgraph System Event Notifier

Instructions:
1. You MUST make sure sources/etc/dbus-1/system.d/com.Subgraph.EventNotifier.conf is in /etc/dbus-1/system.d
2. You might also need to do run the following after compilation: /sbin/paxctl -cm ./eventnotifier

Please always make sure that your copy of shw700/sublogmon is also up-to-date; this project communicates with it via DBus and they must be expecting to produce and consume the exact same data structure(s) in order to work properly.
