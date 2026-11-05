Unicode true

####
## Kisetsu's Windows installer (ADR 0046). Built by `make windows`, which
## passes the binary and the versions:
##   makensis -DARG_WAILS_AMD64_BINARY=../../bin/kisetsu.exe \
##            -DINFO_PRODUCTVERSION=0.3.0 -DKISETSU_VERSION=0.3.0 project.nsi
##
## Per user, without administrator rights: into
## %LocalAppData%\Programs\Kisetsu, listed in "Apps & features". Removing it
## never deletes the user's library (%LocalAppData%\Kisetsu).
####
!define INFO_PROJECTNAME    "kisetsu"
!define INFO_COMPANYNAME    "Miroslav Sentivanac"
!define INFO_PRODUCTNAME    "Kisetsu"
!define INFO_COPYRIGHT      "© 2026 Miroslav Sentivanac. MIT License."
!define PRODUCT_EXECUTABLE  "kisetsu.exe"
!define UNINST_KEY_NAME     "io.github.sentivanac.Kisetsu"
!define REQUEST_EXECUTION_LEVEL "user"
!define WAILS_INSTALL_SCOPE "user"

!ifndef INFO_PRODUCTVERSION
  !error "Pass -DINFO_PRODUCTVERSION=x.y.z (make windows does)"
!endif
!ifndef KISETSU_VERSION
  !define KISETSU_VERSION "${INFO_PRODUCTVERSION}"
!endif

# The agent's value in the Run key (internal/autostart).
!define RUN_KEY   "Software\Microsoft\Windows\CurrentVersion\Run"
!define RUN_VALUE "io.github.sentivanac.Kisetsu.Agent"

!include "wails_tools.nsh"

# The version information for these two must consist of 4 parts
VIProductVersion "${INFO_PRODUCTVERSION}.0"
VIFileVersion    "${INFO_PRODUCTVERSION}.0"

VIAddVersionKey "CompanyName"     "${INFO_COMPANYNAME}"
VIAddVersionKey "FileDescription" "${INFO_PRODUCTNAME} Installer"
VIAddVersionKey "ProductVersion"  "${INFO_PRODUCTVERSION}"
VIAddVersionKey "FileVersion"     "${INFO_PRODUCTVERSION}"
VIAddVersionKey "LegalCopyright"  "${INFO_COPYRIGHT}"
VIAddVersionKey "ProductName"     "${INFO_PRODUCTNAME}"

# Enable HiDPI support. https://nsis.sourceforge.io/Reference/ManifestDPIAware
ManifestDPIAware true

!include "MUI.nsh"

!define MUI_ICON "..\icon.ico"
!define MUI_UNICON "..\icon.ico"
!define MUI_ABORTWARNING
!define MUI_FINISHPAGE_RUN "$INSTDIR\${PRODUCT_EXECUTABLE}"
!define MUI_FINISHPAGE_RUN_TEXT "Start Kisetsu"

!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH

!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES

!insertmacro MUI_LANGUAGE "English"

Name "${INFO_PRODUCTNAME}"
OutFile "..\..\bin\kisetsu-${KISETSU_VERSION}-windows-amd64-setup.exe"
InstallDir "$LOCALAPPDATA\Programs\${INFO_PRODUCTNAME}"
ShowInstDetails show

Function .onInit
   !insertmacro wails.checkArchitecture
FunctionEnd

# stopKisetsu ends the window and the agent of this user, so their files
# can be replaced or removed. The agent keeps no state that needs a clean
# exit.
!macro stopKisetsu
    nsExec::Exec 'taskkill /F /IM ${PRODUCT_EXECUTABLE}'
    Pop $0
    Sleep 500
!macroend

Section
    !insertmacro wails.setShellContext

    !insertmacro wails.webview2runtime

    !insertmacro stopKisetsu

    SetOutPath $INSTDIR

    !insertmacro wails.files
    File "/oname=LICENSE.txt" "..\..\..\LICENSE"
    File "/oname=THIRD_PARTY_LICENSES.txt" "..\..\bin\THIRD_PARTY_LICENSES.txt"

    CreateShortcut "$SMPROGRAMS\${INFO_PRODUCTNAME}.lnk" "$INSTDIR\${PRODUCT_EXECUTABLE}"
    CreateShortCut "$DESKTOP\${INFO_PRODUCTNAME}.lnk" "$INSTDIR\${PRODUCT_EXECUTABLE}"

    !insertmacro wails.writeUninstaller

    # After an update: start the agent again if rotation or the tray icon
    # was on (its Run value exists).
    ReadRegStr $0 HKCU "${RUN_KEY}" "${RUN_VALUE}"
    StrCmp $0 "" +2
        Exec '"$INSTDIR\${PRODUCT_EXECUTABLE}" agent'
SectionEnd

Section "uninstall"
    !insertmacro wails.setShellContext

    !insertmacro stopKisetsu

    # Kisetsu's own entries, never the user's data.
    DeleteRegValue HKCU "${RUN_KEY}" "${RUN_VALUE}"
    RMDir /r "$LOCALAPPDATA\Kisetsu\cache\webview"
    RMDir /r "$AppData\${PRODUCT_EXECUTABLE}" # WebView2 cache of builds before 0.3.0

    RMDir /r $INSTDIR

    Delete "$SMPROGRAMS\${INFO_PRODUCTNAME}.lnk"
    Delete "$DESKTOP\${INFO_PRODUCTNAME}.lnk"

    !insertmacro wails.deleteUninstaller

    DetailPrint "Your library and settings were kept in $LOCALAPPDATA\Kisetsu."
    DetailPrint "Delete that folder to remove them as well."
SectionEnd
