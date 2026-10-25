param(
    [Parameter(Mandatory = $true)][string]$Executable
)
$ErrorActionPreference = 'Stop'
$target = (Resolve-Path -LiteralPath $Executable).Path
$icon = Join-Path (Split-Path $PSScriptRoot -Parent) 'web\assets\app.ico'

# Windows resource group 1 is also used by MyGO for the window/taskbar icon.
if (-not ('OhMyMC.BuildResources' -as [type])) {
    Add-Type -TypeDefinition @'
using System;
using System.ComponentModel;
using System.IO;
using System.Runtime.InteropServices;

namespace OhMyMC {
    public static class BuildResources {
        [DllImport("kernel32.dll", CharSet = CharSet.Unicode, SetLastError = true)]
        private static extern IntPtr BeginUpdateResource(string path, bool deleteExisting);
        [DllImport("kernel32.dll", CharSet = CharSet.Unicode, SetLastError = true)]
        private static extern bool UpdateResource(IntPtr handle, IntPtr type, IntPtr name, ushort language, byte[] data, uint length);
        [DllImport("kernel32.dll", SetLastError = true)]
        private static extern bool EndUpdateResource(IntPtr handle, bool discard);

        public static void Embed(string executable, string icon) {
            byte[] source = File.ReadAllBytes(icon);
            using (var reader = new BinaryReader(new MemoryStream(source)))
            using (var group = new MemoryStream())
            using (var writer = new BinaryWriter(group)) {
                if (reader.ReadUInt16() != 0 || reader.ReadUInt16() != 1)
                    throw new InvalidDataException("Invalid ICO header.");
                ushort count = reader.ReadUInt16();
                if (count == 0 || count > 256) throw new InvalidDataException("Invalid ICO image count.");
                writer.Write((ushort)0); writer.Write((ushort)1); writer.Write(count);
                var images = new byte[count][];
                for (int index = 0; index < count; index++) {
                    byte[] entry = reader.ReadBytes(16);
                    if (entry.Length != 16) throw new InvalidDataException("Truncated ICO directory.");
                    uint length = BitConverter.ToUInt32(entry, 8), offset = BitConverter.ToUInt32(entry, 12);
                    if (length == 0 || (ulong)offset + length > (ulong)source.Length)
                        throw new InvalidDataException("Invalid ICO image bounds.");
                    images[index] = new byte[length];
                    Buffer.BlockCopy(source, (int)offset, images[index], 0, (int)length);
                    writer.Write(entry, 0, 12); writer.Write((ushort)(index + 1));
                }
                IntPtr handle = BeginUpdateResource(executable, false);
                if (handle == IntPtr.Zero) throw new Win32Exception(Marshal.GetLastWin32Error());
                try {
                    for (int index = 0; index < count; index++) {
                        if (!UpdateResource(handle, (IntPtr)3, (IntPtr)(index + 1), 0, images[index], (uint)images[index].Length))
                            throw new Win32Exception(Marshal.GetLastWin32Error());
                    }
                    byte[] bytes = group.ToArray();
                    if (!UpdateResource(handle, (IntPtr)14, (IntPtr)1, 0, bytes, (uint)bytes.Length))
                        throw new Win32Exception(Marshal.GetLastWin32Error());
                    IntPtr commit = handle; handle = IntPtr.Zero;
                    if (!EndUpdateResource(commit, false)) throw new Win32Exception(Marshal.GetLastWin32Error());
                } finally {
                    if (handle != IntPtr.Zero) EndUpdateResource(handle, true);
                }
            }
        }
    }
}
'@
}
[OhMyMC.BuildResources]::Embed($target, $icon)
