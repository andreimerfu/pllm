"use client"

import * as React from "react"
import { Plus, Users, User } from "lucide-react"
import { Button } from "../ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "../ui/dialog"
import { Input } from "../ui/input"
import { Label } from "../ui/label"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "../ui/select"
import { Separator } from "../ui/separator"
import { Card, CardContent, CardHeader, CardTitle } from "../ui/card"
import { Badge } from "../ui/badge"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "../ui/tabs"

interface CreateKeyDialogProps {
  isAdmin: boolean
  userTeams: any[]
  onCreateKey: (keyData: any) => void
}

export function CreateKeyDialog({ isAdmin, userTeams, onCreateKey }: CreateKeyDialogProps) {
  const [open, setOpen] = React.useState(false)
  const [formData, setFormData] = React.useState({
    name: "",
    ownership: "user",
    teamId: "",
    expiration: "never",
    maxBudget: "",
    budgetPeriod: "monthly",
    tpm: "",
    rpm: "",
    enableAdvanced: false,
  })

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    
    let keyData: any = {
      name: formData.name || 'New API Key',
      max_budget: formData.maxBudget ? parseFloat(formData.maxBudget) : undefined,
      budget_duration: formData.budgetPeriod,
      tpm: formData.tpm ? parseInt(formData.tpm) : undefined,
      rpm: formData.rpm ? parseInt(formData.rpm) : undefined,
    }

    // Handle ownership for admin users
    if (isAdmin) {
      keyData.key_type = formData.ownership === 'system' ? 'system' : 'api'
    } else {
      keyData.key_type = 'api'
      if (formData.teamId) {
        keyData.team_id = formData.teamId
      }
    }

    // Handle expiration
    if (formData.expiration !== 'never') {
      const days = parseInt(formData.expiration)
      const expiresAt = new Date()
      expiresAt.setDate(expiresAt.getDate() + days)
      keyData.expires_at = expiresAt.toISOString()
    }

    onCreateKey(keyData)
    setOpen(false)
    
    // Reset form
    setFormData({
      name: "",
      ownership: "user",
      teamId: "",
      expiration: "never",
      maxBudget: "",
      budgetPeriod: "monthly",
      tpm: "",
      rpm: "",
      enableAdvanced: false,
    })
  }

  const getBudgetPeriodText = (period: string) => {
    switch (period) {
      case 'daily': return 'day'
      case 'weekly': return 'week'  
      case 'monthly': return 'month'
      case 'yearly': return 'year'
      default: return 'month'
    }
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        <Button>
          <Plus className="mr-2 h-4 w-4" />
          {isAdmin ? 'Generate Key' : 'Create API Key'}
        </Button>
      </DialogTrigger>
      <DialogContent className="max-w-2xl max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>Create New API Key</DialogTitle>
          <DialogDescription>
            Configure your new API key with custom settings and permissions.
          </DialogDescription>
        </DialogHeader>

        <form onSubmit={handleSubmit} className="space-y-6">
          {/* Basic Settings */}
          <div className="space-y-4">
            <div>
              <Label htmlFor="name">Key Name *</Label>
              <Input
                id="name"
                value={formData.name}
                onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                placeholder="Enter a descriptive name"
                required
              />
            </div>

            {/* Ownership */}
            {isAdmin ? (
              <div>
                <Label>Key Type</Label>
                <Select 
                  value={formData.ownership} 
                  onValueChange={(value) => setFormData({ ...formData, ownership: value })}
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="user">
                      <div className="flex items-center gap-2">
                        <User className="h-4 w-4" />
                        User Key
                      </div>
                    </SelectItem>
                    <SelectItem value="team">
                      <div className="flex items-center gap-2">
                        <Users className="h-4 w-4" />
                        Team Key
                      </div>
                    </SelectItem>
                    <SelectItem value="system">
                      <div className="flex items-center gap-2">
                        <Badge variant="outline">SYS</Badge>
                        System Key
                      </div>
                    </SelectItem>
                  </SelectContent>
                </Select>
              </div>
            ) : userTeams.length > 0 ? (
              <div>
                <Label>Key Type</Label>
                <Select 
                  value={formData.teamId || "personal"} 
                  onValueChange={(value) => setFormData({ ...formData, teamId: value === "personal" ? "" : value })}
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="personal">
                      <div className="flex items-center gap-2">
                        <User className="h-4 w-4" />
                        Personal Key
                      </div>
                    </SelectItem>
                    {userTeams.map((team) => (
                      <SelectItem key={team.id} value={team.id}>
                        <div className="flex items-center gap-2">
                          <Users className="h-4 w-4" />
                          {team.name} Team
                        </div>
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            ) : null}
          </div>

          <Separator />

          {/* Settings Tabs */}
          <Tabs defaultValue="basic" className="space-y-4">
            <TabsList className="grid w-full grid-cols-3">
              <TabsTrigger value="basic">Basic</TabsTrigger>
              <TabsTrigger value="limits">Limits</TabsTrigger>
              <TabsTrigger value="advanced">Advanced</TabsTrigger>
            </TabsList>

            <TabsContent value="basic" className="space-y-4">
              <div>
                <Label>Expiration</Label>
                <Select 
                  value={formData.expiration} 
                  onValueChange={(value) => setFormData({ ...formData, expiration: value })}
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="never">Never expires</SelectItem>
                    <SelectItem value="7">7 days</SelectItem>
                    <SelectItem value="30">30 days</SelectItem>
                    <SelectItem value="90">90 days</SelectItem>
                    <SelectItem value="365">1 year</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            </TabsContent>

            <TabsContent value="limits" className="space-y-4">
              <div className="grid gap-4">
                <div>
                  <Label htmlFor="maxBudget">Budget Limit ($)</Label>
                  <Input
                    id="maxBudget"
                    type="number"
                    step="0.01"
                    min="0"
                    value={formData.maxBudget}
                    onChange={(e) => setFormData({ ...formData, maxBudget: e.target.value })}
                    placeholder="Leave empty for unlimited"
                  />
                </div>

                <div>
                  <Label>Budget Period</Label>
                  <Select 
                    value={formData.budgetPeriod} 
                    onValueChange={(value) => setFormData({ ...formData, budgetPeriod: value })}
                  >
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="daily">Daily</SelectItem>
                      <SelectItem value="weekly">Weekly</SelectItem>
                      <SelectItem value="monthly">Monthly</SelectItem>
                      <SelectItem value="yearly">Yearly</SelectItem>
                    </SelectContent>
                  </Select>
                  {formData.maxBudget && (
                    <p className="text-sm text-muted-foreground mt-1">
                      ${formData.maxBudget} per {getBudgetPeriodText(formData.budgetPeriod)}
                    </p>
                  )}
                </div>
              </div>
            </TabsContent>

            <TabsContent value="advanced" className="space-y-4">
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <Label htmlFor="tpm">TPM Limit</Label>
                  <Input
                    id="tpm"
                    type="number"
                    min="0"
                    value={formData.tpm}
                    onChange={(e) => setFormData({ ...formData, tpm: e.target.value })}
                    placeholder="100000"
                  />
                  <p className="text-xs text-muted-foreground mt-1">Tokens per minute</p>
                </div>

                <div>
                  <Label htmlFor="rpm">RPM Limit</Label>
                  <Input
                    id="rpm"
                    type="number"
                    min="0"
                    value={formData.rpm}
                    onChange={(e) => setFormData({ ...formData, rpm: e.target.value })}
                    placeholder="60"
                  />
                  <p className="text-xs text-muted-foreground mt-1">Requests per minute</p>
                </div>
              </div>
            </TabsContent>
          </Tabs>

          {/* Configuration Preview */}
          <Card>
            <CardHeader className="pb-3">
              <CardTitle className="text-base">Configuration Preview</CardTitle>
            </CardHeader>
            <CardContent className="space-y-2 text-sm">
              <div className="flex justify-between">
                <span className="text-muted-foreground">Name:</span>
                <span>{formData.name || 'Untitled Key'}</span>
              </div>
              <div className="flex justify-between">
                <span className="text-muted-foreground">Type:</span>
                <span className="capitalize">
                  {isAdmin ? formData.ownership : (formData.teamId ? 'Team' : 'Personal')}
                </span>
              </div>
              <div className="flex justify-between">
                <span className="text-muted-foreground">Expires:</span>
                <span>
                  {formData.expiration === 'never' ? 'Never' : `${formData.expiration} days`}
                </span>
              </div>
              {formData.maxBudget && (
                <div className="flex justify-between">
                  <span className="text-muted-foreground">Budget:</span>
                  <span>${formData.maxBudget}/{getBudgetPeriodText(formData.budgetPeriod)}</span>
                </div>
              )}
              {(formData.tpm || formData.rpm) && (
                <div className="flex justify-between">
                  <span className="text-muted-foreground">Rate Limits:</span>
                  <span>
                    {formData.tpm && `${formData.tpm} TPM`}
                    {formData.tpm && formData.rpm && ', '}
                    {formData.rpm && `${formData.rpm} RPM`}
                  </span>
                </div>
              )}
            </CardContent>
          </Card>

          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => setOpen(false)}>
              Cancel
            </Button>
            <Button type="submit">
              Create API Key
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}