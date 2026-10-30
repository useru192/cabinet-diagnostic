HOW TO USE
  1. Copy this WHOLE folder to the Windows PC (extract the .zip first).
  2. Connect the device by USB.
  3. Double-click getsysteminfo.bat.
  4. Wait 1-2 minutes. Keep the USB connected; do not close the window.

RESULT
  A file named  mem_report_<serial>_<date>_<time>.txt
  is created in this folder. Send that file back.

  Example: mem_report_0123456789ABCDEF_20261028_143022.txt

NOTES
  - getsysteminfo.bat and the platform-tools folder must stay together.
  - Do not run the .bat while it is still inside a .zip.
  - Do not use ( ) or other special characters in the folder name.

TROUBLESHOOTING
  "AdbWinApi.dll was not found" / "No usable adb found"
      -> platform-tools folder is missing or incomplete. Re-copy it.

  "No device detected"
      -> Check the USB cable, USB debugging, and the ADB driver.

  "root permission not obtained"
      -> Normal on non-rooted builds. The report is still generated.
