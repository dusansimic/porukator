# Protobuf javalite keeps generated message classes via reflection on field
# names; keep them and their builders.
-keep class porukator.v1.** { *; }
-keepclassmembers class * extends com.google.protobuf.GeneratedMessageLite { *; }
