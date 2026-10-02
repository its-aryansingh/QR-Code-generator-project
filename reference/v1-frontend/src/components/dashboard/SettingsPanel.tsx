import React, { useState } from "react";
import axios from "axios";
import { toast } from "sonner";
import { useAuthStore } from "@/lib/auth";

interface SettingsPanelProps {
    user: any;
    accessToken: string | null;
    API_BASE_URL: string;
}

export const SettingsPanel: React.FC<SettingsPanelProps> = ({
    user,
    accessToken,
    API_BASE_URL,
}) => {
    const [formData, setFormData] = useState({
        name: user?.name || "",
        currentPassword: "",
        newPassword: "",
        confirmPassword: "",
    });
    const [loading, setLoading] = useState(false);

    const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
        setFormData({ ...formData, [e.target.name]: e.target.value });
    };

    const handleUpdateProfile = async (e: React.FormEvent) => {
        e.preventDefault();
        if (!accessToken) return;

        if (formData.newPassword && formData.newPassword !== formData.confirmPassword) {
            toast.error("New passwords do not match");
            return;
        }

        setLoading(true);
        try {
            const payload: any = {
                name: formData.name,
            };
            if (formData.newPassword) {
                payload.password = formData.newPassword;
            }

            await axios.put(`${API_BASE_URL}/api/v1/user/profile`, payload, {
                headers: { Authorization: `Bearer ${accessToken}` },
            });

            toast.success("Profile updated successfully");
            // Optionally update local user state if needed, request re-login if password changed?
            // For now just success message.
            setFormData({ ...formData, currentPassword: "", newPassword: "", confirmPassword: "" });
        } catch (err: any) {
            console.error("Profile update error:", err);
            toast.error(err.response?.data?.error || "Failed to update profile");
        } finally {
            setLoading(false);
        }
    };

    return (
        <div className="max-w-2xl mx-auto space-y-8">
            <div className="bg-[#1e1e2d] border border-white/10 rounded-2xl p-8">
                <h2 className="text-2xl font-bold text-white mb-6">Account Settings</h2>

                <form onSubmit={handleUpdateProfile} className="space-y-6">
                    {/* Profile Info */}
                    <div className="space-y-4">
                        <h3 className="text-lg font-medium text-slate-300">Profile Information</h3>
                        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                            <div>
                                <label className="block text-sm font-medium text-slate-400 mb-1">Email</label>
                                <input
                                    type="email"
                                    value={user?.email || ""}
                                    disabled
                                    className="w-full bg-black/20 border border-white/5 rounded-xl px-4 py-3 text-slate-500 cursor-not-allowed"
                                />
                            </div>
                            <div>
                                <label className="block text-sm font-medium text-slate-400 mb-1">Full Name</label>
                                <input
                                    type="text"
                                    name="name"
                                    value={formData.name}
                                    onChange={handleChange}
                                    className="w-full bg-black/20 border border-white/10 rounded-xl px-4 py-3 text-white focus:outline-none focus:border-violet-500/50"
                                    placeholder="Your Name"
                                />
                            </div>
                        </div>
                    </div>

                    <div className="h-px bg-white/5 my-6"></div>

                    {/* Password Change */}
                    <div className="space-y-4">
                        <h3 className="text-lg font-medium text-slate-300">Change Password</h3>
                        <p className="text-sm text-slate-500">Leave blank if you don't want to change it.</p>

                        <div className="space-y-4">
                            <div>
                                <label className="block text-sm font-medium text-slate-400 mb-1">New Password</label>
                                <input
                                    type="password"
                                    name="newPassword"
                                    value={formData.newPassword}
                                    onChange={handleChange}
                                    className="w-full bg-black/20 border border-white/10 rounded-xl px-4 py-3 text-white focus:outline-none focus:border-violet-500/50"
                                    placeholder="New Password (min 8 chars)"
                                />
                            </div>
                            <div>
                                <label className="block text-sm font-medium text-slate-400 mb-1">Confirm New Password</label>
                                <input
                                    type="password"
                                    name="confirmPassword"
                                    value={formData.confirmPassword}
                                    onChange={handleChange}
                                    className="w-full bg-black/20 border border-white/10 rounded-xl px-4 py-3 text-white focus:outline-none focus:border-violet-500/50"
                                    placeholder="Confirm New Password"
                                />
                            </div>
                        </div>
                    </div>

                    <div className="pt-4">
                        <button
                            type="submit"
                            disabled={loading}
                            className="px-6 py-3 bg-violet-600 hover:bg-violet-700 text-white rounded-xl font-medium transition-colors disabled:opacity-50"
                        >
                            {loading ? "Saving..." : "Save Changes"}
                        </button>
                    </div>
                </form>
            </div>

            {/* Subscription Info (Read Only) */}
            <div className="bg-[#1e1e2d] border border-white/10 rounded-2xl p-8">
                <h3 className="text-lg font-medium text-white mb-4">Subscription Plan</h3>
                <div className="flex items-center justify-between p-4 bg-white/5 rounded-xl border border-white/5">
                    <div>
                        <p className="text-white font-medium capitalize">{user?.plan || "Free"} Plan</p>
                        <p className="text-sm text-slate-500">
                            {user?.plan === "free" ? "Upgrade to unlock more features." : "Thanks for being a pro user!"}
                        </p>
                    </div>
                    {user?.plan === "free" && (
                        <button className="px-4 py-2 bg-gradient-to-r from-amber-500 to-orange-500 text-white text-sm font-medium rounded-lg hover:shadow-lg transition-all">
                            Upgrade
                        </button>
                    )}
                </div>
            </div>
        </div>
    );
};
